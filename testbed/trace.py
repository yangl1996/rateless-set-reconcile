#!/usr/local/bin/python3

import re
import argparse
import glob
from datetime import datetime

parser = argparse.ArgumentParser()
parser.add_argument("PREFIX", help="prefix of the files", type=str)
parser.add_argument("N", help="index of the node", type=int)
parser.add_argument("--cw-trace", help="print the codeword count trace", default=False, action='store_true')
args = parser.parse_args()

filename = args.PREFIX + "-" + str(args.N)
peercnt = 0
peers = {}
cwcnt = []
txcnt = []
gencnt = []
txdelay = [0, 0, 0, 0]  # p5 p50 p95 mean
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
# take the second pass to gather data
with open(filename) as f:
    for line in f:
        if "received cws" in line:
            dt = int(datetime.strptime(line[0:19], '%Y/%m/%d %H:%M:%S').timestamp())
            start = line.find("peer") + 5
            end = line.find(" received")
            peeridx = peers[line[start:end]]
            start = line.find("cws") + 4
            end = line.find("\n")
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
# compute the total number of codewords
if args.cw_trace:
    minLen = 10000000000
    for i in range(peercnt):
        if len(cwcnt[i]) < minLen:
            minLen = len(cwcnt[i])
    last = None
    for t in range(minLen):
        tot = 0
        for pidx in range(peercnt):
            tot += cwcnt[pidx][t]
        if not last is None:
            print(t, tot-last)
        last = tot

