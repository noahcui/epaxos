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
	"os"
	"runtime"
	"sort"
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
var path *string = flag.String("path", "raw_data", "path to store raw data")
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
var totalout int
var successful []int
var ct []uint64
var rarray []int
var rsp []bool
var outInfos []*outInfo
var start []map[int32]time.Time
var end []map[int32]time.Time
var values []string
var numValues = 1024
var weight [genericsmrproto.WEIGHTSIZE]byte

func clientWriter(idx int, writerList []*bufio.Writer, stop chan int, next chan int, wg *sync.WaitGroup) {
	if writerList == nil {
		fmt.Println("stopping nil sender groups ", idx)
		return
	}
	fmt.Println(writerList)
	args := genericsmrproto.Propose{0 /* id */, state.Command{state.PUT, 0, 0}, 0 /* timestamp */, weight}
	for id := int32(0); ; id++ {
		select {
		case i := <-stop:
			writerList[i%N] = nil
			stop := true
			for _, writer := range writerList {
				if writer != nil {
					stop = false
				}
			}
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
			now := time.Now()
			args.Timestamp = now.UnixNano()
			// Determine operation type
			if *writes > rand.Intn(100) {
				args.Command.Op = state.PUT // write operation
			} else {
				args.Command.Op = state.GET // read operation
			}
			err := writerList[sid].WriteByte(genericsmrproto.PROPOSE)
			if err != nil {
				writerList[sid] = nil
			}
			args.Marshal(writerList[sid])
			err = writerList[sid].Flush()
			if err != nil {
				writerList[sid] = nil
			}
			mu.Lock()
			totalout += 1
			mu.Unlock()
			time.Sleep(time.Nanosecond * time.Duration(*thinktime*1000*1000))
		}
	}
}

func clientReader(idx int, reader *bufio.Reader, stop chan int, next chan int, wg *sync.WaitGroup) {
	defer wg.Done()
	if reader == nil {
		return
	}
	var reply genericsmrproto.ProposeReplyTS
	ticker := time.NewTicker(time.Second * time.Duration(*t))
	for {
		select {
		case <-ticker.C:
			stop <- idx
			return
		default:
			if err := reply.Unmarshal(reader); err != nil {
				log.Println("Error when reading:", err)
				stop <- idx
				return
			}

			if reply.OK != 0 {
				ct[idx]++
				mu.Lock()
				if _, notempty := end[idx/N][reply.CommandId]; !notempty {
					end[idx][reply.CommandId] = time.Now()
					start[idx][reply.CommandId] = time.Unix(0, reply.Timestamp)
				}
				mu.Unlock()
			}
		}
	}
	stop <- idx
}

var maxindex int

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

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

	start = make([]map[int32]time.Time, *T*N)
	end = make([]map[int32]time.Time, *T*N)
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
	starttime := time.Now()
	j := 0
	for i := 0; i < *T; i++ {
		done := make(chan int, N)
		next := make(chan int, N)

		for _, reader := range readers[i] {
			start[j] = make(map[int32]time.Time)
			end[j] = make(map[int32]time.Time)
			ct[j] = 0
			go clientReader(j, reader, done, next, &wg)
			wg.Add(1)
			j++
		}

		go clientWriter(i, writers[i], done, next, &wg)
	}
	fmt.Println("testing started!")
	wg.Wait()
	fmt.Println("testing finished, dealing with datas!")
	print := true
	file, err := os.Create(*path)
	if err != nil {
		fmt.Println(err)
		print = false
	}
	defer file.Close()

	w := bufio.NewWriter(file)

	time_now := time.Now()
	var total uint64
	maxindex = -1
	total = 0
	var sum time.Duration
	sum = 0
	total = 0
	// latencies:= make(heap)
	var ls []time.Duration
	onesecondslides := make(map[int][]time.Duration)
	alllatency := make([]time.Duration, 0)
	index := 0
	for idx, endtimes := range end {
		for cid, etime := range endtimes {
			mu.Lock()
			stime := start[idx][cid]
			mu.Unlock()
			l := etime.Sub(stime)
			// fmt.Println(stime, "||", etime)
			ls = append(ls, l)
			sum += l
			total += 1
			index, err = getindex(starttime, etime)
			if err != nil {
				fmt.Println(err)
				return
			}
			if onesecondslides[index] == nil {
				onesecondslides[index] = make([]time.Duration, 0)
			}

			if print {
				line := fmt.Sprintln(stime.UnixNano(), etime.UnixNano())
				fmt.Fprintln(w, line)
			}

			onesecondslides[index] = append(onesecondslides[index], l)
			alllatency = append(alllatency, l)
		}
	}
	if print {
		w.Flush()
	}

	now2 := time.Now()
	fmt.Println("after", now2.Sub(time_now), "start sorting data!")
	alllatency = sortlatency(alllatency)
	i90 := 90 * (len(alllatency) - 1) / 100
	i95 := 95 * (len(alllatency) - 1) / 100
	i99 := 99 * (len(alllatency) - 1) / 100
	i50 := 50 * (len(alllatency) - 1) / 100
	i999 := 999 * (len(alllatency) - 1) / 1000
	imax := len(alllatency) - 1
	x := total / uint64(*t)
	avg := sum / time.Duration(total)
	p90 := alllatency[i90]
	p95 := alllatency[i95]
	p99 := alllatency[i99]
	p50 := alllatency[i50]
	p999 := alllatency[i999]
	pmax := alllatency[imax]
	pmin := alllatency[0]
	now3 := time.Now()
	fmt.Println("after", now3.Sub(now2), "overall sorting finished")

	fmt.Printf("num of clients: %v\nx: %v \nnum of total commands: %v \navg latency: %v \n totalout: %v, p90=%v, p95=%v, p99=%v\n\n\n", *T, x, total, avg, totalout, p90, p95, p99)

	fmt.Println("------------------------------------------------------")
	fmt.Println("DETAILED RESULTS(second, latencies in ms, x)")
	fmt.Println("------------------------------------------------------")
	maxindex = getmaxindex(onesecondslides)
	fmt.Println("sec, 50, 90, 95, 99, 999, max, min, avg, x")
	for i := 0; i <= maxindex; i++ {
		onesecondslides[i] = sortlatency(onesecondslides[i])
		i50 := 50 * (len(onesecondslides[i]) - 1) / 100
		i90 := 95 * (len(onesecondslides[i]) - 1) / 100
		i95 := 95 * (len(onesecondslides[i]) - 1) / 100
		i99 := 99 * (len(onesecondslides[i]) - 1) / 100
		i999 := 999 * (len(onesecondslides[i]) - 1) / 1000
		imax := len(onesecondslides[i]) - 1
		items := onesecondslides[i]
		var subsum time.Duration
		var subtotal uint64
		subsum = 0
		subtotal = 0
		if items != nil {
			for _, item := range items {
				subsum += item
				subtotal += 1
			}
			sp50 := onesecondslides[i][i50]
			sp90 := onesecondslides[i][i90]
			sp95 := onesecondslides[i][i95]
			sp99 := onesecondslides[i][i99]
			sp999 := onesecondslides[i][i999]
			spmax := onesecondslides[i][imax]
			spmin := onesecondslides[i][0]
			spavg := subsum / time.Duration(subtotal)
			sx := subtotal
			fmt.Printf("%v,%v,%v,%v,%v,%v,%v,%v,%v,%v\n",
				i,
				float64(sp50.Nanoseconds())/1000000.0,
				float64(sp90.Nanoseconds())/1000000.0,
				float64(sp95.Nanoseconds())/1000000.0,
				float64(sp99.Nanoseconds())/1000000.0,
				float64(sp999.Nanoseconds())/1000000.0,
				float64(spmax.Nanoseconds())/1000000.0,
				float64(spmin.Nanoseconds())/1000000.0,
				float64(spavg.Nanoseconds())/1000000.0,
				sx)
		} else {
			fmt.Println(i, ",", "nil")
		}
	}

	fmt.Println("Concurrency =", *T)
	fmt.Println("Benchmark Time =", *t)
	fmt.Println("overallx =", x)
	fmt.Println("overallmax =", float64(pmax.Nanoseconds())/1000000.0)
	fmt.Println("overallmin =", float64(pmin.Nanoseconds())/1000000.0)
	fmt.Println("overall50 =", float64(p50.Nanoseconds())/1000000.0)
	fmt.Println("overall90 =", float64(p90.Nanoseconds())/1000000.0)
	fmt.Println("overall95 =", float64(p95.Nanoseconds())/1000000.0)
	fmt.Println("overall99 =", float64(p99.Nanoseconds())/1000000.0)
	fmt.Println("overall999 =", float64(p999.Nanoseconds())/1000000.0)
	fmt.Println("overallavg =", float64(avg.Nanoseconds())/1000000.0)
	master.Close()

}

func sortlatency(inarray []time.Duration) []time.Duration {
	// sort.Ints(inarray)
	sort.Slice(inarray, func(i, j int) bool { return inarray[i] < inarray[j] })
	// len := len(inarray)
	// for i := 0; i < len-1; i++ {
	// 	for j := 0; j < len-i-1; j++ {
	// 		if inarray[j] > inarray[j+1] {
	// 			tmp := inarray[j]
	// 			inarray[j] = inarray[j+1]
	// 			inarray[j+1] = tmp
	// 		}
	// 	}
	// }
	return inarray
}

func getmaxindex(m map[int][]time.Duration) int {
	max := 0
	for idx, _ := range m {
		if idx > max {
			max = idx
		}
	}
	return max
}

func getindex(start time.Time, now time.Time) (int, error) {
	index := 0
	for {
		now = now.Add(-time.Second)
		if now.Before(start) {
			return index, nil
		}
		index++
		if index > 9999 {
			return -1, fmt.Errorf("idx bigger than 9999\n")
		}
	}
	return -1, nil
}
