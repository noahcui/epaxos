package client_ycsb;

import java.io.*;
import java.net.*;
import java.nio.ByteBuffer;
import java.util.*;
import java.util.regex.Pattern;
import java.util.regex.Matcher;

public class client {
    public class GetReplicaListReply {
        private List<String> replicaList;
        private boolean ready;
        private long leaderIdx;

        public GetReplicaListReply(List<String> replicaList, boolean ready, long idx) {
            this.replicaList = replicaList;
            this.ready = ready;
            this.leaderIdx = idx;
        }

        public List<String> getReplicaList() {
            return replicaList;
        }

        public boolean isReady() {
            return ready;
        }

        public long leaderIdx() {
            return leaderIdx;
        }
    }

    public GetReplicaListReply fromJsonString(String jsonString) {
        Pattern replicaListPattern = Pattern.compile("\"ReplicaList\":\\[(.*?)\\]");
        Matcher replicaListMatcher = replicaListPattern.matcher(jsonString);
        List<String> replicaList = new ArrayList<String>();
        while (replicaListMatcher.find()) {
            String[] parts = replicaListMatcher.group(1).split(",");
            for (String part : parts) {
                replicaList.add(part.trim().replaceAll("\"", ""));
            }
        }

        Pattern readyPattern = Pattern.compile("\"Ready\":(true|false)");
        Matcher readyMatcher = readyPattern.matcher(jsonString);
        boolean ready = false;
        if (readyMatcher.find()) {
            ready = Boolean.parseBoolean(readyMatcher.group(1));
        }

        Pattern leaderIdxPattern = Pattern.compile("\"Leader\":(\\d+)");
        Matcher leaderIdxMatcher = leaderIdxPattern.matcher(jsonString);
        long idx = 0;
        if (leaderIdxMatcher.find()) {
            idx = Long.parseLong(leaderIdxMatcher.group(1));
        }

        return new GetReplicaListReply(replicaList, ready, idx);
    }

    Socket cli;

    public static Socket NewClient(String addr) throws Exception {
        String[] parts = addr.split(":");
        String ip = parts[0];
        int port = Integer.parseInt(parts[1]);
        InetAddress address = InetAddress.getByName(ip);
        Socket socket = new Socket("", port);
        return socket;
    }

    GetReplicaListReply resp;

    public void init(String masterAddr, int masterPort) {
        try {
            URL url = new URL("http://localhost:8080/replicaList");
            HttpURLConnection conn = (HttpURLConnection) url.openConnection();
            conn.setRequestMethod("POST");
            conn.setDoOutput(true);

            String requestParam = "{\"methodName\":\"Master.GetReplicaList\",\"body\":\"{\\\"ReplicaList\\\":[],\\\"Ready\\\":false,\\\"Leader\\\":0}\"}";

            Writer writer = new OutputStreamWriter(conn.getOutputStream(), "UTF-8");
            writer.write(requestParam);
            writer.close();

            BufferedReader reader = new BufferedReader(new InputStreamReader(conn.getInputStream(), "UTF-8"));
            StringBuilder response = new StringBuilder();
            String line;
            while ((line = reader.readLine()) != null) {
                response.append(line);
            }
            reader.close();

            String jsonResponse = response.toString();

            System.out.println(jsonResponse);
            resp = fromJsonString(jsonResponse);
            String[] addrs = resp.replicaList.toArray(new String[resp.replicaList.size()]);
            cli = NewClient(addrs[(int) resp.leaderIdx]);
            // System.out.println(addrs[(int) resp.leaderIdx]);
        } catch (Exception e) {
            System.out.println(e.getMessage());
        } finally {
            // System.out.println("finally");
        }
    }

    private static class Propose implements Serializable {
        private int commandId;
        private Command command;
        private long timestamp;

        public Propose(int commandId, Command command, long timestamp) {
            this.commandId = commandId;
            this.command = command;
            this.timestamp = timestamp;
        }

        public void marshal(OutputStream out) throws IOException {
            // byte[] b =
            byte[] bs = new byte[8];
            byte[] b = new byte[4];
            int tmp32 = commandId;
            byte[] start = new byte[1];
            start[0] = 0;
            out.write(start, 0, 1);
            b[0] = (byte) (tmp32);
            b[1] = (byte) (tmp32 >> 8);
            b[2] = (byte) (tmp32 >> 16);
            b[3] = (byte) (tmp32 >> 24);
            out.write(b, 0, 4);

            // Write command
            command.marshal(out);

            long tmp64 = timestamp;
            bs[0] = (byte) (tmp64);
            bs[1] = (byte) (tmp64 >> 8);
            bs[2] = (byte) (tmp64 >> 16);
            bs[3] = (byte) (tmp64 >> 24);
            bs[4] = (byte) (tmp64 >> 32);
            bs[5] = (byte) (tmp64 >> 40);
            bs[6] = (byte) (tmp64 >> 48);
            bs[7] = (byte) (tmp64 >> 56);
            out.write(bs);
            System.out.println(timestamp);
        }
    }

    private static class ProposeReplyTS implements Serializable {
        private byte ok;
        private int commandId;
        private String value;
        private long timestamp;

        public void unmarshal(DataInputStream wire) throws IOException {
            byte[] bs = new byte[8];

            wire.readFully(bs, 0, 5);
            ok = bs[0];
            commandId = ((bs[1] & 0xFF) | ((bs[2] & 0xFF) << 8) | ((bs[3] & 0xFF) << 16) | ((bs[4] & 0xFF) << 24));

            wire.readFully(bs, 0, 8);

            long vl = ByteBuffer.wrap(bs).getLong();
            byte[] data = new byte[(int) vl];

            wire.readFully(data, 0, (int) vl);
            value = new String(data);
            System.out.println(vl);
            wire.readFully(bs, 0, 8);

            timestamp = ((long) (bs[0] & 0xFF) | ((long) (bs[1] & 0xFF) << 8) | ((long) (bs[2] & 0xFF) << 16)
                    | ((long) (bs[3] & 0xFF) << 24) | ((long) (bs[4] & 0xFF) << 32) | ((long) (bs[5] & 0xFF) << 40)
                    | ((long) (bs[6] & 0xFF) << 48) | ((long) (bs[7] & 0xFF) << 56));
        }
    }

    private static class Command implements Serializable {
        private byte op;
        private long k;
        private String v;

        public Command(byte op, long k, String v) {
            this.op = op;
            this.k = k;
            this.v = v;
        }

        public void marshal(OutputStream out) throws IOException {
            ByteBuffer buffer = ByteBuffer.allocate(8);

            // Write op
            out.write(op);

            // Write k
            buffer.putLong(0, k);
            out.write(buffer.array());

            // Write v length
            long vl = v.length();
            buffer.putLong(0, vl);
            out.write(buffer.array());

            // Write v
            out.write(v.getBytes());
        }
    }

    public void sendRequestAndAwaitForReply() {

        try {
            OutputStream outToServer = cli.getOutputStream();
            InputStream inFromServer = cli.getInputStream();

            // Create and send Propose
            Command cmd = new Command((byte) 2, 12345, "test");
            Propose propose = new Propose(2588, cmd, System.currentTimeMillis());
            DataOutputStream dos = new DataOutputStream(outToServer);
            propose.marshal(dos);
            System.out.println(dos.size());
            dos.flush();

            System.out.println("out finished");
            // Receive ProposeReplyTS
            DataInputStream dis = new DataInputStream(inFromServer);
            ProposeReplyTS reply = new ProposeReplyTS();
            reply.unmarshal(dis);
            System.out.println(reply.ok);
            System.out.println(reply.commandId);
            System.out.println(reply.timestamp);
            System.out.println(reply.value);
        } catch (IOException e) {
            e.printStackTrace();
        }
    }

    public void cleanup() {
        // do nothing
    }

    public void read() {
        sendRequestAndAwaitForReply();
        // do nothing
    }

    public void update() {
        // do nothing
    }

    public void insert() {
        // do nothing
    }

    public void delete() {
        // do nothing
    }

    public static void main(String[] args) {
        client myMaster = new client();
        myMaster.init("", 7087);
        // System.out.println(myMaster.resp.leaderIdx);
        myMaster.sendRequestAndAwaitForReply();
    }
}