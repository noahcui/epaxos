package menciusopt

import (
	"dlog"
	"encoding/binary"
	"fastrpc"
	"fmt"
	"genericsmr"
	"genericsmrproto"
	"io"
	"log"
	"menciusoptproto"
	"state"
	"time"
)

const CHAN_BUFFER_SIZE = 200000
const WAIT_BEFORE_SKIP_MS = 50
const NB_INST_TO_SKIP = 100
const MAX_SKIPS_WAITING = 20
const TRUE = uint8(1)
const FALSE = uint8(0)

type Replica struct {
	*genericsmr.Replica      // extends a generic Paxos replica
	skipChan                 chan fastrpc.Serializable
	prepareChan              chan fastrpc.Serializable
	acceptChan               chan fastrpc.Serializable
	commitChan               chan fastrpc.Serializable
	prepareReplyChan         chan fastrpc.Serializable
	acceptReplyChan          chan fastrpc.Serializable
	delayedSkipChan          chan *DelayedSkip
	skipRPC                  uint8
	prepareRPC               uint8
	acceptRPC                uint8
	commitRPC                uint8
	prepareReplyRPC          uint8
	acceptReplyRPC           uint8
	clockChan                chan bool   // clock
	instanceSpace            []*Instance // the space of all instances (used and not yet used)
	crtInstance              int32       // highest active instance number that this replica knows about
	latestInstReady          int32       // highest instance number that is in the READY state (ready to commit)
	latestInstCommitted      int32       // highest instance number (owned by the current replica) that was committed
	blockingInstance         int32       // the lowest instance that could block commits
	noCommitFor              int
	waitingToCommitSomething bool
	Shutdown                 bool
	skipsWaiting             int
	counter                  int
	skippedTo                []int32
	maxcommit                int32
}

type DelayedSkip struct {
	skipEnd int32
}

type InstanceStatus int

const (
	PREPARING InstanceStatus = iota
	ACCEPTED
	READY
	COMMITTED
	EXECUTED
)

type Instance struct {
	skipped       bool
	nbInstSkipped int
	commands      []state.Command
	ballot        int32
	status        InstanceStatus
	lb            *LeaderBookkeeping
}

type LeaderBookkeeping struct {
	clientProposal *genericsmr.Propose
	maxRecvBallot  int32
	prepareOKs     int
	acceptOKs      int
	nacks          int
}

func (r *Replica) updatelatestInstCommitted(slot int32) {
	if r.latestInstCommitted >= slot {
		return
	}
	r.latestInstCommitted = slot
}

func NewReplica(id int, peerAddrList []string, thrifty bool, exec bool, dreply bool, durable bool) *Replica {
	skippedTo := make([]int32, len(peerAddrList))
	for i := 0; i < len(skippedTo); i++ {
		skippedTo[i] = -1
	}
	r := &Replica{genericsmr.NewReplica(id, peerAddrList, thrifty, exec, dreply),
		make(chan fastrpc.Serializable, genericsmr.CHAN_BUFFER_SIZE*4),
		make(chan fastrpc.Serializable, genericsmr.CHAN_BUFFER_SIZE),
		make(chan fastrpc.Serializable, genericsmr.CHAN_BUFFER_SIZE),
		make(chan fastrpc.Serializable, genericsmr.CHAN_BUFFER_SIZE),
		make(chan fastrpc.Serializable, genericsmr.CHAN_BUFFER_SIZE),
		make(chan fastrpc.Serializable, genericsmr.CHAN_BUFFER_SIZE*4),
		make(chan *DelayedSkip, genericsmr.CHAN_BUFFER_SIZE),
		0, 0, 0, 0, 0, 0,
		make(chan bool, 10),
		make([]*Instance, 10*1024*1024),
		int32(id),
		int32(-1),
		int32(0),
		int32(0),
		0,
		false,
		false,
		0,
		0,
		skippedTo,
		0}

	r.Durable = durable

	r.skipRPC = r.RegisterRPC(new(menciusoptproto.Skip), r.skipChan)
	r.prepareRPC = r.RegisterRPC(new(menciusoptproto.Prepare), r.prepareChan)
	r.acceptRPC = r.RegisterRPC(new(menciusoptproto.Accept), r.acceptChan)
	r.commitRPC = r.RegisterRPC(new(menciusoptproto.Commit), r.commitChan)
	r.prepareReplyRPC = r.RegisterRPC(new(menciusoptproto.PrepareReply), r.prepareReplyChan)
	r.acceptReplyRPC = r.RegisterRPC(new(menciusoptproto.AcceptReply), r.acceptReplyChan)

	go r.run()

	return r
}

//append a log entry to stable storage
func (r *Replica) recordInstanceMetadata(inst *Instance) {
	if !r.Durable {
		return
	}

	var b [10]byte
	if inst.skipped {
		b[0] = 1
	} else {
		b[0] = 0
	}
	binary.LittleEndian.PutUint32(b[1:5], uint32(inst.nbInstSkipped))
	binary.LittleEndian.PutUint32(b[5:9], uint32(inst.ballot))
	b[9] = byte(inst.status)
	r.StableStore.Write(b[:])
}

//write a sequence of commands to stable storage
func (r *Replica) recordCommand(cmd *state.Command) {
	if !r.Durable {
		return
	}

	if cmd == nil {
		return
	}
	cmd.Marshal(io.Writer(r.StableStore))
}

//sync with the stable store
func (r *Replica) sync() {
	if !r.Durable {
		return
	}

	r.StableStore.Sync()
}

func (r *Replica) replyPrepare(replicaId int32, reply *menciusoptproto.PrepareReply) {
	r.SendMsg(replicaId, r.prepareReplyRPC, reply)
}

func (r *Replica) replyAccept(replicaId int32, reply *menciusoptproto.AcceptReply) {
	r.SendMsg(replicaId, r.acceptReplyRPC, reply)
}

/* ============= */

/* Main event processing loop */
var lastSeenInstance int32

func (r *Replica) run() {
	r.ConnectToPeers()

	dlog.Println("Waiting for client connections")

	go r.WaitForClientConnections()

	if r.Exec {
		log.Printf("executing!\n")
		// fmt.Println("makesure changes applied, hello")
		go r.executeCommands()
	}
	bTicker := time.NewTicker(time.Millisecond * 1000)
	go r.clock()
	for !r.Shutdown {

		select {
		case <-bTicker.C:
			if r.instanceSpace[r.crtInstance] == nil {
				r.instanceSpace[r.crtInstance] = &Instance{false,
					0,
					make([]state.Command, 1),
					r.makeBallotLargerThan(0),
					ACCEPTED,
					&LeaderBookkeeping{nil, 0, 0, 0, 0}}
			}
			fmt.Println(r.crtInstance, r.latestInstReady)
			// TODO: broadcast the msg and reset the batch log#
			r.recordInstanceMetadata(r.instanceSpace[r.crtInstance])
			r.bcastAccept(r.crtInstance, r.instanceSpace[r.crtInstance].ballot, FALSE, 0, r.instanceSpace[r.crtInstance].commands)
			r.sync()
			r.crtInstance += int32(r.N)

		case propose := <-r.ProposeChan:
			//got a Propose from a client
			dlog.Printf("Proposal with id %d\n", propose.CommandId)
			r.handlePropose(propose)
			break

		case skipS := <-r.skipChan:
			skip := skipS.(*menciusoptproto.Skip)
			//got a Skip from another replica
			dlog.Printf("Skip for instances %d-%d\n", skip.StartInstance, skip.EndInstance)
			r.handleSkip(skip)

		case prepareS := <-r.prepareChan:
			prepare := prepareS.(*menciusoptproto.Prepare)
			//got a Prepare message
			dlog.Printf("Received Prepare from replica %d, for instance %d\n", prepare.LeaderId, prepare.Instance)
			r.handlePrepare(prepare)
			break

		case acceptS := <-r.acceptChan:
			accept := acceptS.(*menciusoptproto.Accept)
			//got an Accept message
			dlog.Printf("Received Accept from replica %d, for instance %d\n", accept.LeaderId, accept.Instance)
			r.handleAccept(accept)
			break

		case commitS := <-r.commitChan:
			commit := commitS.(*menciusoptproto.Commit)
			//got a Commit message
			dlog.Printf("Received Commit from replica %d, for instance %d\n", commit.LeaderId, commit.Instance)
			r.handleCommit(commit)
			break

		case prepareReplyS := <-r.prepareReplyChan:
			prepareReply := prepareReplyS.(*menciusoptproto.PrepareReply)
			//got a Prepare reply
			dlog.Printf("Received PrepareReply for instance %d\n", prepareReply.Instance)
			r.handlePrepareReply(prepareReply)
			break

		case acceptReplyS := <-r.acceptReplyChan:
			acceptReply := acceptReplyS.(*menciusoptproto.AcceptReply)
			//got an Accept reply
			dlog.Printf("Received AcceptReply for instance %d\n", acceptReply.Instance)
			r.handleAcceptReply(acceptReply)
			break

		case delayedSkip := <-r.delayedSkipChan:
			r.handleDelayedSkip(delayedSkip)
			break

		case <-r.clockChan:
			if lastSeenInstance == r.blockingInstance {
				r.noCommitFor++
			} else {
				r.noCommitFor = 0
				lastSeenInstance = r.blockingInstance
			}
			if r.noCommitFor >= 50+int(r.Id) && r.crtInstance >= r.blockingInstance+int32(r.N) {
				r.noCommitFor = 0
				dlog.Printf("Doing force commit\n")
				r.forceCommit()
			}
			break
		}
	}
}

func (r *Replica) clock() {
	for !r.Shutdown {
		time.Sleep(100 * 1000 * 1000)
		r.clockChan <- true
	}
}

func (r *Replica) makeUniqueBallot(ballot int32) int32 {
	return (ballot << 4) | r.Id
}

func (r *Replica) makeBallotLargerThan(ballot int32) int32 {
	return r.makeUniqueBallot((ballot >> 4) + 1)
}

var sk menciusoptproto.Skip

func (r *Replica) bcastPrepare(instance int32, ballot int32) {
	defer func() {
		if err := recover(); err != nil {
			dlog.Println("Prepare bcast failed:", err)
		}
	}()
	args := &menciusoptproto.Prepare{r.Id, instance, ballot}

	n := r.N - 1
	if r.Thrifty {
		n = r.N >> 1
	}
	q := r.Id

	for sent := 0; sent < n; {
		q = (q + 1) % int32(r.N)
		if q == r.Id {
			break
		}
		if !r.Alive[q] {
			continue
		}
		sent++
		r.SendMsg(q, r.prepareRPC, args)
	}
}

var ma menciusoptproto.Accept

func (r *Replica) bcastAccept(instance int32, ballot int32, skip uint8, nbInstToSkip int32, commands []state.Command) {
	defer func() {
		if err := recover(); err != nil {
			dlog.Println("Accept bcast failed:", err)
		}
	}()
	ma.LeaderId = r.Id
	ma.Instance = instance
	ma.Ballot = ballot
	ma.Skip = skip
	ma.NbInstancesToSkip = nbInstToSkip
	ma.Commands = commands
	args := &ma
	//args := &menciusoptproto.Accept{r.Id, instance, ballot, skip, nbInstToSkip, command}

	n := r.N - 1
	q := r.Id

	sent := 0
	for sent < n {
		q = (q + 1) % int32(r.N)
		if q == r.Id {
			break
		}
		if !r.Alive[q] {
			continue
		}
		if r.Thrifty {
			inst := (instance/int32(r.N))*int32(r.N) + q
			if inst > instance {
				inst -= int32(r.N)
			}
			if inst < 0 || r.instanceSpace[inst] != nil {
				continue
			}
		}
		sent++
		r.SendMsg(q, r.acceptRPC, args)
	}

	for sent < r.N>>1 {
		q = (q + 1) % int32(r.N)
		if q == r.Id {
			break
		}
		if !r.Alive[q] {
			continue
		}
		if r.Thrifty {
			inst := (instance/int32(r.N))*int32(r.N) + q
			if inst > instance {
				inst -= int32(r.N)
			}
			if inst >= 0 && r.instanceSpace[inst] == nil {
				continue
			}
		}
		sent++
		r.SendMsg(q, r.acceptRPC, args)
	}
}

var mc menciusoptproto.Commit

func (r *Replica) bcastCommit(instance int32, skip uint8, nbInstToSkip int32, commands []state.Command) {
	defer func() {
		if err := recover(); err != nil {
			dlog.Println("Commit bcast failed:", err)
		}
	}()
	mc.LeaderId = r.Id
	mc.Instance = instance
	mc.Skip = skip
	mc.NbInstancesToSkip = nbInstToSkip
	mc.Commands = commands
	//args := &menciusoptproto.Commit{r.Id, instance, skip, nbInstToSkip, command}
	args := &mc

	n := r.N - 1
	q := r.Id

	for sent := 0; sent < n; {
		q = (q + 1) % int32(r.N)
		if q == r.Id {
			break
		}
		if !r.Alive[q] {
			continue
		}
		sent++
		r.SendMsg(q, r.commitRPC, args)
	}
}

func (r *Replica) handlePropose(propose *genericsmr.Propose) {
	instNo := r.crtInstance
	if r.instanceSpace[instNo] == nil {
		r.instanceSpace[instNo] = &Instance{false,
			0,
			make([]state.Command, 1),
			r.makeBallotLargerThan(0),
			ACCEPTED,
			&LeaderBookkeeping{nil, 0, 0, 0, 0}}
	}

	r.instanceSpace[instNo].commands = append(r.instanceSpace[instNo].commands, propose.Command)
	r.recordCommand(&propose.Command)
	r.sync()
	dlog.Printf("Choosing req. %d in instance %d\n", propose.CommandId, instNo)
}

func (r *Replica) handleSkip(skip *menciusoptproto.Skip) {
	r.instanceSpace[skip.StartInstance] = &Instance{true,
		int(skip.EndInstance-skip.StartInstance)/r.N + 1,
		nil,
		0,
		COMMITTED,
		nil}
	r.updateBlocking(skip.StartInstance)
}

func (r *Replica) handlePrepare(prepare *menciusoptproto.Prepare) {
	inst := r.instanceSpace[prepare.Instance]

	if inst == nil {
		dlog.Println("Replying OK to null-instance Prepare")
		r.replyPrepare(prepare.LeaderId, &menciusoptproto.PrepareReply{prepare.Instance,
			TRUE,
			-1,
			FALSE,
			0,
			make([]state.Command, 0)})
		// state.Command{state.NONE, 0, 0}

		r.instanceSpace[prepare.Instance] = &Instance{false,
			0,
			nil,
			prepare.Ballot,
			PREPARING,
			nil}
	} else {
		ok := TRUE
		if prepare.Ballot < inst.ballot {
			ok = FALSE
		}
		if inst.commands == nil {
			inst.commands = make([]state.Command, 0)
			// inst.commands = &state.Command{state.NONE, 0, 0}
		}
		skipped := FALSE
		if inst.skipped {
			skipped = TRUE
		}
		r.replyPrepare(prepare.LeaderId, &menciusoptproto.PrepareReply{prepare.Instance,
			ok,
			inst.ballot,
			skipped,
			int32(inst.nbInstSkipped),
			inst.commands})
	}
}

func (r *Replica) timerHelper(ds *DelayedSkip) {
	time.Sleep(WAIT_BEFORE_SKIP_MS * 1000 * 1000)
	r.delayedSkipChan <- ds
}

func (r *Replica) handleAccept(accept *menciusoptproto.Accept) {
	// flush := true
	inst := r.instanceSpace[accept.Instance]

	if inst != nil && inst.ballot > accept.Ballot {
		r.replyAccept(accept.LeaderId, &menciusoptproto.AcceptReply{accept.Instance, FALSE, inst.ballot, -1, -1})
		return
	}

	skipStart := int32(-1)
	skipEnd := int32(-1)
	if inst == nil {
		skip := false
		if accept.Skip == TRUE {
			skip = true
		}
		r.instanceSpace[accept.Instance] = &Instance{skip,
			int(accept.NbInstancesToSkip),
			accept.Commands,
			accept.Ballot,
			ACCEPTED,
			nil}
		r.recordInstanceMetadata(r.instanceSpace[accept.Instance])
		for _, command := range accept.Commands {
			r.recordCommand(&command)
		}
		r.sync()

		r.replyAccept(accept.LeaderId, &menciusoptproto.AcceptReply{accept.Instance, TRUE, -1, skipStart, skipEnd})
	} else {
		if inst.status == COMMITTED || inst.status == EXECUTED {
			if inst.commands == nil {
				inst.commands = accept.Commands
			}
			dlog.Printf("ATTENTION! Reordered Commit\n")
		} else {
			inst.commands = accept.Commands
			inst.ballot = accept.Ballot
			inst.status = ACCEPTED
			inst.skipped = false
			if accept.Skip == TRUE {
				inst.skipped = true
			}
			inst.nbInstSkipped = int(accept.NbInstancesToSkip)

			r.recordInstanceMetadata(inst)

			r.replyAccept(accept.LeaderId, &menciusoptproto.AcceptReply{accept.Instance, TRUE, inst.ballot, skipStart, skipEnd})
		}
	}
	r.updateBlocking(accept.Instance)
}

func (r *Replica) handleDelayedSkip(delayedSkip *DelayedSkip) {
	r.skipsWaiting--
	for _, w := range r.PeerWriters {
		if w != nil {
			w.Flush()
		}
	}
}

func (r *Replica) handleCommit(commit *menciusoptproto.Commit) {
	inst := r.instanceSpace[commit.Instance]

	dlog.Printf("Committing instance %d\n", commit.Instance)

	if inst == nil {
		skip := false
		if commit.Skip == TRUE {
			skip = true
		}
		r.instanceSpace[commit.Instance] = &Instance{skip,
			int(commit.NbInstancesToSkip),
			make([]state.Command, 1), //&commit.Command,
			0,
			COMMITTED,
			nil}
	} else {
		//inst.command = &commit.Command
		inst.status = COMMITTED
		inst.skipped = false
		if commit.Skip == TRUE {
			inst.skipped = true
		}
		inst.nbInstSkipped = int(commit.NbInstancesToSkip)
		// for _, lb := range inst.lb {
		if inst.lb != nil && inst.lb.clientProposal != nil {
			// try command in the next available instance
			r.ProposeChan <- inst.lb.clientProposal
			inst.lb.clientProposal = nil
		}
		// }

	}

	r.recordInstanceMetadata(r.instanceSpace[commit.Instance])

	if commit.Instance%int32(r.N) == r.Id%int32(r.N) {
		if r.crtInstance < commit.Instance+commit.NbInstancesToSkip*int32(r.N) {
			r.crtInstance = commit.Instance + commit.NbInstancesToSkip*int32(r.N)
		}
	}

	// Try to commit instances waiting for this one
	r.updateBlocking(commit.Instance)
}

func (r *Replica) handlePrepareReply(preply *menciusoptproto.PrepareReply) {
	dlog.Printf("PrepareReply for instance %d\n", preply.Instance)

	inst := r.instanceSpace[preply.Instance]

	if inst.status != PREPARING {
		// we've moved on -- these are delayed replies, so just ignore
		return
	}

	if preply.OK == TRUE {
		// for i, _ := range inst.lb {
		inst.lb.prepareOKs++
		inst.lb.maxRecvBallot = preply.Ballot
		// }
		if preply.Ballot > inst.lb.maxRecvBallot {
			inst.commands = preply.Commands
			inst.skipped = false
			if preply.Skip == TRUE {
				inst.skipped = true
			}
			inst.nbInstSkipped = int(preply.NbInstancesToSkip)

			if inst.lb.prepareOKs+1 > r.N>>1 {
				inst.status = ACCEPTED
				// for i, _ := range inst.lb {
				inst.lb.nacks = 0
				// }

				skip := FALSE
				if inst.skipped {
					skip = TRUE
				}
				r.bcastAccept(preply.Instance, inst.ballot, skip, int32(inst.nbInstSkipped), inst.commands)
			}
		}

	}

}
func (r *Replica) handleAcceptReply(areply *menciusoptproto.AcceptReply) {
	dlog.Printf("AcceptReply for instance %d\n", areply.Instance)

	inst := r.instanceSpace[areply.Instance]

	if areply.OK == TRUE {
		// for i, _ := range inst.lb {
		inst.lb.acceptOKs++
		// }
		if areply.SkippedStartInstance > -1 {
			r.instanceSpace[areply.SkippedStartInstance] = &Instance{true,
				int(areply.SkippedEndInstance-areply.SkippedStartInstance)/r.N + 1,
				nil,
				0,
				COMMITTED,
				nil}
			r.updateBlocking(areply.SkippedStartInstance)
		}

		if inst.status == COMMITTED || inst.status == EXECUTED { //TODO || aargs.Ballot != inst.ballot {
			// we've moved on, these are delayed replies, so just ignore
			return
		}

		if inst.lb.acceptOKs+1 > r.N>>1 {
			if inst.skipped {
				//TODO what if
			}
			inst.status = READY
			if !inst.skipped && areply.Instance > r.latestInstReady {
				r.latestInstReady = areply.Instance
			}
			r.updateBlocking(areply.Instance)
		}
	}
}

func (r *Replica) updateBlocking(instance int32) {
	if instance != r.blockingInstance {
		return
	}

	for r.blockingInstance = r.blockingInstance; true; r.blockingInstance++ {
		if r.blockingInstance <= r.skippedTo[int(r.blockingInstance)%r.N] {
			continue
		}
		if r.instanceSpace[r.blockingInstance] == nil {
			return
		}
		inst := r.instanceSpace[r.blockingInstance]
		if inst.status == COMMITTED && inst.skipped {
			r.skippedTo[int(r.blockingInstance)%r.N] = r.blockingInstance + int32((inst.nbInstSkipped-1)*r.N)
			continue
		}
		if inst.status == ACCEPTED && inst.skipped {
			return
		}
		if r.blockingInstance%int32(r.N) == r.Id || inst.lb != nil {
			if inst.status == READY {
				//commit my instance
				dlog.Printf("Am about to commit instance %d\n", r.blockingInstance)

				inst.status = COMMITTED
				if inst.lb.clientProposal != nil && !r.Dreply {
					// give client the all clear
					dlog.Printf("Sending ACK for req. %d\n", inst.lb.clientProposal.CommandId)
					r.ReplyProposeTS(&genericsmrproto.ProposeReplyTS{TRUE, inst.lb.clientProposal.CommandId, state.NIL, inst.lb.clientProposal.Timestamp},
						inst.lb.clientProposal.Reply)
				}
				skip := FALSE
				if inst.skipped {
					skip = TRUE
				}

				r.recordInstanceMetadata(inst)
				r.sync()

				r.bcastCommit(r.blockingInstance, skip, int32(inst.nbInstSkipped), inst.commands)
			} else if inst.status != COMMITTED && inst.status != EXECUTED {
				return
			}
			if inst.skipped {
				r.skippedTo[int(r.blockingInstance)%r.N] = r.blockingInstance + int32((inst.nbInstSkipped-1)*r.N)
			}
		} else {
			if inst.status == PREPARING || (inst.status == ACCEPTED && inst.skipped) {
				return
			}
		}
	}
}

func (r *Replica) executeCommands() {
	execedUpTo := int32(-1)
	// skippedTo := make([]int32, r.N)
	skippedToOrig := make([]int32, r.N)
	// conflicts := make(map[state.Key]int32, 60000)

	for q := 0; q < r.N; q++ {
		skippedToOrig[q] = -1
	}

	for !r.Shutdown {
		executed := false
		execedUpTo += 1
		for i := execedUpTo; i < r.crtInstance; i++ {
			if r.instanceSpace[execedUpTo] == nil {
				executed = false
				break
			}
			if r.instanceSpace[execedUpTo].status == COMMITTED {

			}
			executed = true
		}
		if !executed {
			// if r.maxcommit-execedUpTo > 10 {
			// 	r.forceCommit()
			// }
			fmt.Println(execedUpTo)
			time.Sleep(time.Millisecond * 1000)
		}
	}
}

func (r *Replica) forceCommit() {
	//find what is the oldest un-initialized instance and try to take over
	problemInstance := r.blockingInstance

	//try to take over the problem instance
	if int(problemInstance)%r.N == int(r.Id+1)%r.N {
		log.Println("Replica", r.Id, "r.N=", r.N, "Trying to take over instance", problemInstance)
		if r.instanceSpace[problemInstance] == nil {
			r.instanceSpace[problemInstance] = &Instance{true,
				NB_INST_TO_SKIP,
				make([]state.Command, 1),
				r.makeUniqueBallot(1),
				PREPARING,
				&LeaderBookkeeping{nil, 0, 0, 0, 0}}
			r.bcastPrepare(problemInstance, r.instanceSpace[problemInstance].ballot)
		} else {
			log.Println("Not nil")
			r.bcastPrepare(problemInstance, r.instanceSpace[problemInstance].ballot)
		}
	}
}
