#!/usr/local/bin/python3

import re
import argparse
import glob
from datetime import datetime
import numpy as np

parser = argparse.ArgumentParser()
parser.add_argument("PREFIX", help="prefix of the files", type=str)
parser.add_argument("-n", help="number of servers", type=int, default=19)
#parser.add_argument("-d", "--delay", help="one-way per-edge delay in ms", type=float, default=70.0)
#parser.add_argument("--name", help="name of the network", type=str, default="Test")
#parser.add_argument("--latency", help="use latency file", type=str, default=None)
args = parser.parse_args()

totgen = 0
tottime = 0
rcvd = []
dl= []
meanDelay = []
overhead = []

for i in range(args.n):
    filename = args.PREFIX + "-" + str(i)
    peercnt = 0
    peers = {}
    cwcnt = []
    txcnt = []
    gencnt = []
    txdelay = [0, 0, 0, 0]  # p5 p50 p95 mean
    cwrate = []
    # take the first pass to find all peers
    with open(filename) as f:
        for line in f:
            if "key exchanged with peer" in line:
                start = line.find("with peer") + 10
                end = line.find(",")
                addr = line[start:end]
                peers[addr] = peercnt
                peercnt += 1
                cwcnt.append([])
                cwrate.append(0)
    # take the second pass to gather data
    with open(filename) as f:
        started = False
        for line in f:
            if not started and "data logging warmup completed" in line:
                started = True
            if started:
                if "received cws" in line:
                    dt = int(datetime.strptime(line[0:19], '%Y/%m/%d %H:%M:%S').timestamp())
                    start = line.find("peer") + 5
                    end = line.find(" received")
                    peeridx = peers[line[start:end]]
                    start = line.find("cws") + 4
                    end = line.find(" last second")
                    cnt = int(line[start:end])
                    cwcnt[peeridx].append(cnt)
                elif "tx=" in line:
                    dt = int(datetime.strptime(line[0:19], '%Y/%m/%d %H:%M:%S').timestamp())
                    start = line.find("tx=") + 3
                    end = line.find(", p5_latency_ms")
                    cnt = int(line[start:end])
                    txcnt.append(cnt)
                    start = line.find("p5_latency_ms=") + 14
                    end = line.find(", p95")
                    txdelay[0] = float(line[start:end])
                    start = line.find("p95_latency_ms=") + 15
                    end = line.find(", p50")
                    txdelay[2] = float(line[start:end])
                    start = line.find("p50_latency_ms=") + 15
                    end = line.find(", mean")
                    txdelay[1] = float(line[start:end])
                    start = line.find("mean_latency_ms=") + 16
                    end = line.find("\n")
                    txdelay[3] = float(line[start:end])
                elif "generated tx" in line:
                    dt = int(datetime.strptime(line[0:19], '%Y/%m/%d %H:%M:%S').timestamp())
                    start = line.find("tx") + 3
                    end = line.find("\n")
                    cnt = int(line[start:end])
                    gencnt.append(cnt)
                elif "codeword rate" in line:
                    start = line.find("peer") + 5
                    end = line.find(" codeword rate")
                    peeridx = peers[line[start:end]]
                    start = line.find("rate") + 5
                    end = line.find("dropped")
                    rate = float(line[start:end])
                    cwrate[peeridx] = rate
    # compute the total number of codewords
    minLen = 10000000000
    for i in range(peercnt):
        if len(cwcnt[i]) < minLen:
            minLen = len(cwcnt[i])
    if len(txcnt) < minLen:
        minLen = len(txcnt)
    if len(gencnt) < minLen:
        minLen = len(gencnt)
    startCw = 0
    endCw = 0
    for i in range(peercnt):
        startCw += cwcnt[i][0]
        endCw += cwcnt[i][minLen-1]
    startTx = txcnt[0]
    endTx = txcnt[minLen-1]
    startGen = gencnt[0]
    endGen = gencnt[minLen-1]
    # overhead (cw/tx), received tx rate, latency p5, p50, p95, mean
    print(float(endCw-startCw) / float(endTx-startTx+endGen-startGen), float(endTx-startTx+endGen-startGen) / float(minLen), txdelay[0], txdelay[1], txdelay[2], txdelay[3])
    totgen += (gencnt[minLen-1]-gencnt[0])
    rcvd.append(endTx-startTx+endGen-startGen)
    tottime += minLen
    overhead.append(float(endCw-startCw) / float(endTx-startTx+endGen-startGen))
    meanDelay.append(txdelay[3] * (endTx-startTx) / (endTx-startTx+endGen-startGen))

for v in rcvd:
    dl.append(v / totgen)

print("per-node generation rate:", totgen/(tottime/args.n)/args.n)
print("deliverability p5 mean p95:", np.quantile(dl, 0.05), np.mean(dl), np.quantile(dl, 0.95))
print("overhead p5 mean p95:", np.quantile(overhead, 0.05), np.mean(overhead), np.quantile(overhead, 0.95))
print("average delay p5 mean p95:", np.quantile(meanDelay, 0.05)/1000, np.mean(meanDelay)/1000, np.quantile(meanDelay, 0.95)/1000)
print(totgen/(tottime/args.n), np.quantile(dl, 0.05), np.mean(dl), np.quantile(dl, 0.95), np.quantile(overhead, 0.05), np.mean(overhead), np.quantile(overhead, 0.95), np.quantile(meanDelay, 0.05)/1000, np.mean(meanDelay)/1000, np.quantile(meanDelay, 0.95)/1000)
