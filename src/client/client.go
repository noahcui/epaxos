package main

import (
	"bufio"
	"flag"
	"fmt"
	"genericsmrproto"
	"log"
	"masterproto"
	"math/rand"
	"net"
	"net/rpc"
	"runtime"
	"state"
	"sync"
	"time"
)

var outstandingReqs = flag.Int64("or", 1, "Number of outstanding requests a thread can have at any given time.")

var masterAddr *string = flag.String("maddr", "", "Master address. Defaults to localhost")
var masterPort *int = flag.Int("mport", 7087, "Master port.  Defaults to 7077.")
var reqsNb *int = flag.Int("q", 5000, "Total number of requests. Defaults to 5000.")
var writes *int = flag.Int("w", 100, "Percentage of updates (writes). Defaults to 100%.")
var noLeader *bool = flag.Bool("e", false, "Egalitarian (no leader). Defaults to false.")
var fast *bool = flag.Bool("f", false, "Fast Paxos: send message directly to all replicas. Defaults to false.")
var rounds *int = flag.Int("r", 10, "Split the total number of requests into this many rounds, and do rounds sequentially. Defaults to 1.")
var procs *int = flag.Int("p", -1, "GOMAXPROCS. Defaults to unlimit")
var check = flag.Bool("check", false, "Check that every expected reply was received exactly once.")
var eps *int = flag.Int("eps", 0, "Send eps more messages per round than the client will wait for (to discount stragglers). Defaults to 0.")
var conflicts *int = flag.Int("c", -1, "Percentage of conflicts. Defaults to 0%")
var t *int = flag.Int("t", 5, "time in seconds for each round")
var T = flag.Int("T", 10, "Number of threads (simulated clients).")
var s = flag.Float64("s", 2, "Zipfian s parameter")
var v = flag.Float64("v", 1, "Zipfian v parameter")
var thinktime *int = flag.Int("think", 100, "time in nanoseconds for thinking")
var zKeys = flag.Uint64("z", 1e9, "Number of unique keys in zipfian distribution.")
var theta = flag.Float64("theta", 0.99, "Theta zipfian parameter")

type outInfo struct {
	// sync.Mutex
	// sema       *semaphore.Weighted // Controls number of outstanding operations
	startTimes time.Time // The time at which operations were sent out
	endTimes   time.Time
	done       bool
}

var mu sync.Mutex

var N int

var successful []int
var ct []uint64
var rarray []int
var rsp []bool
var outInfos []*outInfo
var start []map[int32]time.Time
var end []map[int32]time.Time

func clientWriter(idx int, writerList []*bufio.Writer, stop chan int, next chan int, wg *sync.WaitGroup) {
	// defer wg.Done()
	if writerList == nil {
		fmt.Println("stopping nil sender groups ", idx)
		return
	}
	fmt.Println(writerList)
	args := genericsmrproto.Propose{0 /* id */, state.Command{state.PUT, 0, 0}, 0 /* timestamp */}
	for id := int32(0); ; id++ {
		select {
		case i := <-stop:
			fmt.Println("stopping sender ", idx)
			writerList[i%N] = nil
			stop := true
			for _, writer := range writerList {
				if writer != nil {
					fmt.Println("stopping sender ", i)
					stop = false
				}
			}
			fmt.Println("all connections are nil, stopping sender", idx)
			if stop {
				return
			}
		default:

			// for {
			sid := rand.Int() % N
			writer := writerList[sid]
			for writer == nil {
				sid = rand.Int() % N
				writer = writerList[sid]
				if writer != nil {
					// fmt.Printf("redirecting msg from %v to a new server, %v, %v\n", sid, writerList, writer)
					break
				}
			}
			// fmt.Println("now sending", id)
			args.CommandId = id
			r := int(id) % 10000
			if r < *conflicts {
				args.Command.K = 42
			} else {
				args.Command.K = state.Key(r)
			}
			// Determine operation type
			if *writes > rand.Intn(100) {
				args.Command.Op = state.PUT // write operation
			} else {
				args.Command.Op = state.GET // read operation
			}
			err := writerList[sid].WriteByte(genericsmrproto.PROPOSE)
			if err != nil {
				fmt.Println(sid, err)
				writerList[sid] = nil
			}
			args.Marshal(writerList[sid])
			err = writerList[sid].Flush()
			if err != nil {
				fmt.Println(sid, err)
				writerList[sid] = nil
			}
			// fmt.Println(idx, id)
			mu.Lock()
			start[idx][id] = time.Now()
			mu.Unlock()
			time.Sleep(time.Nanosecond * time.Duration(*thinktime))
			// break
			// }

			// out := outInfo{startTimes: time.Now()}

		}
	}
}

func clientReader(idx int, reader *bufio.Reader, stop chan int, next chan int, wg *sync.WaitGroup) {
	defer wg.Done()
	if reader == nil {
		fmt.Println("stopping nil reader", idx)
		return
	}
	var reply genericsmrproto.ProposeReply
	ticker := time.NewTicker(time.Second * time.Duration(*t))
	for {
		select {
		case <-ticker.C:
			fmt.Println("stopping reader ", idx)
			stop <- idx
			fmt.Println(idx, "sent out")
			return
		default:
			// fmt.Println("for new msg!")
			if err := reply.Unmarshal(reader); err != nil {
				log.Println("Error when reading:", err)
				stop <- idx
				return
			}
			if reply.OK != 0 {
				ct[idx]++
				mu.Lock()
				if _, notempty := end[idx/N][reply.CommandId]; !notempty {
					end[idx/N][reply.CommandId] = time.Now()
				}
				mu.Unlock()
				// fmt.Println(1 / 2)
			}
		}
	}
	stop <- idx
}

// func makeConnections()
func main() {
	flag.Parse()

	runtime.GOMAXPROCS(*procs)
	if *conflicts > 100 {
		log.Fatalf("Conflicts percentage must be between 0 and 100.\n")
	}

	master, err := rpc.DialHTTP("tcp", fmt.Sprintf("%s:%d", *masterAddr, *masterPort))
	if err != nil {
		log.Fatalf("Error connecting to master\n")
	}

	rlReply := new(masterproto.GetReplicaListReply)
	err = master.Call("Master.GetReplicaList", new(masterproto.GetReplicaListArgs), rlReply)
	if err != nil {
		log.Fatalf("Error making the GetReplicaList RPC")
	}
	var wg sync.WaitGroup
	N = len(rlReply.ReplicaList)
	readers := make([][]*bufio.Reader, *T)
	writers := make([][]*bufio.Writer, *T)

	start = make([]map[int32]time.Time, *T)
	end = make([]map[int32]time.Time, *T)
	for i := 0; i < *T; i++ {
		readers[i] = make([]*bufio.Reader, 0)
		writers[i] = make([]*bufio.Writer, 0)
		for j := 0; j < N; j++ {
			server, err := net.Dial("tcp", rlReply.ReplicaList[j])
			if err != nil {
				log.Println("error connecting to server ", j)
				// i--
				readers[i] = append(readers[i], nil)
				writers[i] = append(writers[i], nil)
				continue
			}
			reader := bufio.NewReader(server)
			writer := bufio.NewWriter(server)
			readers[i] = append(readers[i], reader)
			writers[i] = append(writers[i], writer)
		}
	}

	ct = make([]uint64, *T*N)
	// fmt.Println("start testing! waiting for results")
	j := 0
	for i := 0; i < *T; i++ {
		done := make(chan int, N)
		next := make(chan int, N)
		start[i] = make(map[int32]time.Time)
		end[i] = make(map[int32]time.Time)
		for _, reader := range readers[i] {
			ct[j] = 0
			go clientReader(j, reader, done, next, &wg)
			wg.Add(1)
			j++
		}

		go clientWriter(i, writers[i], done, next, &wg)
	}
	fmt.Println("testing started!")
	wg.Wait()
	var total uint64
	total = 0
	var sum time.Duration
	// latencies:= make(heap)
	var ls []time.Duration
	for idx, endtimes := range end {
		for cid, etime := range endtimes {
			mu.Lock()
			stime := start[idx][cid]
			mu.Unlock()
			l := etime.Sub(stime)
			ls = append(ls, l)
			sum += l
			total += 1
		}
	}
	fmt.Printf("num of clients: %v\nx: %v \nnum of total commands: %v \navg latency: %v \n\n\n", *T, total/uint64(*t), total, sum/time.Duration(total))
	master.Close()
	// 	for i := 0; i < *T; i++ {
	// 		readers[i].Reset(nil)
	// 		writers[i].Reset(nil)
	// 	}
}
