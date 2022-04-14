package main

import (
	"runtime"
	"github.com/yangl1996/rateless-set-reconcile/ldpc"
	"log"
	"net"
	"math/rand"
	"time"
	"flag"
	"strings"
	"encoding/binary"
	"github.com/DataDog/sketches-go/ddsketch"
)

func getDelayUs(t *ldpc.Transaction) float64 {
	dt := t.Serialized()
	sent := int64(binary.LittleEndian.Uint64(dt[0:8]))
	rcvd := time.Now().UnixMicro()
	return float64(rcvd - sent)
}

func randomTransaction() *ldpc.Transaction {
	ut := uint64(time.Now().UnixMicro())
    d := ldpc.TransactionData{}
	binary.LittleEndian.PutUint64(d[0:8], ut)
    rand.Read(d[8:])
    t := &ldpc.Transaction{}
    t.UnmarshalBinary(d[:])
    return t
}

func main() {
	rand.Seed(time.Now().Unix())

	warmup := flag.Duration("warmup", time.Duration(1)*time.Minute, "time to wait before collecting data")
	addr := flag.String("l", ":9000", "address to listen")
	conn := flag.String("p", "", "comma-delimited list of addresses to connect to")
	K := flag.Uint64("k", 50, "coding window size and max codeword degree")
	M := flag.Uint64("m", 262144, "peeling window size")
	C := flag.Float64("c", 0.03, "parameter C of the soliton distribution")
	txRate := flag.Float64("tx", 1000.0, "local transaction generation rate")
	delta := flag.Float64("delta", 0.5, "parameter delta of the soliton distribution")
	initRate := flag.Float64("r0", 0, "initial codeword rate, match tx rate if zero")
	minRate := flag.Float64("rmin", 1.0, "min codeword rate")
	incConstant := flag.Float64("inc", 0.1, "codeword rate increment upon loss")
	targetLoss := flag.Float64("loss", 0.02, "target codeword loss rate")
	decodeTimeout := flag.Duration("t", 500 * time.Millisecond, "codeword decoding timeout")
	flag.Parse()

	flag.VisitAll(func(f *flag.Flag) {
		log.Printf("config name %v value %v\n", f.Name, f.Value)
	})


	if *initRate == 0 {
		*initRate = *txRate
	}
	if *initRate < *minRate {
		*initRate = *minRate
	}

	sketch, err := ddsketch.NewDefaultDDSketchWithExactSummaryStatistics(0.001)
	if err != nil {
		log.Fatalln(err)
	}
	c := &controller {
		newPeer: make(chan *peer),
		decodedTransaction: make(chan *ldpc.Transaction, 1000),
		localTransaction: make(chan *ldpc.Transaction, 1000),
		K: *K,
		M: *M,
		solitonC: *C,
		solitonDelta: *delta,
		initRate: *initRate,
		minRate: *minRate,
		incConstant: *incConstant,
		targetLoss: *targetLoss,
		decodeTimeout: *decodeTimeout,
		delaySketch: sketch,
		warmupTime: *warmup,
	}

	go c.loop()
	l, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalln("failed to listen:", err)
	}
	go func() {
		for {
			cn, err := l.Accept()
			if err != nil {
				log.Println("error accepting incoming connection:", err)
			} else {
				c.handleConn(cn.RemoteAddr().String(), cn)
			}
		}
	}()

	if *conn != "" {
		addrList := strings.Split(*conn, ",")
		for _, a := range addrList {
			go func(addr string) {
				var cn net.Conn
				var err error
				for {
					cn, err = net.Dial("tcp", addr)
					if err != nil {
						log.Println("error connecting:", err)
						time.Sleep(time.Duration(1 * time.Second))
					} else {
						break
					}
				}
				c.handleConn(cn.RemoteAddr().String(), cn)
			}(a)
		}
	}

	if *txRate > 0 {
		r := *txRate
		go func() {
			cnt := 0
			timer := time.NewTimer(time.Duration(rand.ExpFloat64() / r * float64(time.Second)))
			ticker := time.NewTicker(time.Duration(1) * time.Second)
			for {
				select {
				case <-ticker.C:
					log.Printf("generated tx %d\n", cnt)
				case <-timer.C:
					timer.Reset(time.Duration(rand.ExpFloat64() / r * float64(time.Second)))
					tx := randomTransaction()
					c.localTransaction <- tx
					cnt += 1
				}
			}
		}()
	}

	log.Println("running")

	var m runtime.MemStats
	for {
		runtime.ReadMemStats(&m)
		log.Printf("Heap=%v MB, Sys=%v MB, GCCycles=%v\n", m.Alloc/1024/1024, m.Sys/1024/1024, m.NumGC)
		time.Sleep(1 * time.Second)
	}
}
