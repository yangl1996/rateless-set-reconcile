#!/usr/local/bin/python3

import re
import argparse
import glob
import json
from datetime import datetime

parser = argparse.ArgumentParser()
parser.add_argument("PREFIX", help="prefix of the files", type=str)
parser.add_argument("-n", help="number of servers", type=int, default=19)
parser.add_argument("-s", help="server file", type=str, default="servers.json")
args = parser.parse_args()

addrToIdx = {}

rates = {}

with open(args.s, 'r') as f:
  config = json.load(f)
  for i in range(len(config)):
      s = config[i]
      addrToIdx[s['PublicIP']] = i

for i in range(args.n):
    filename = args.PREFIX + "-" + str(i)
    peercnt = 0
    cwrate = {}
    with open(filename) as f:
        dst = {}
        started = False
        for line in f:
            if not started and "data logging warmup completed" in line:
                started = True
            if started:
                if "codeword rate" in line:
                    start = line.find("peer") + 5
                    end = line.find(" codeword rate")
                    ip = line[start:end].split(":")[0]
                    peerIdx = addrToIdx[ip]
                    start = line.find("rate") + 5
                    end = line.find("dropped")
                    rate = float(line[start:end])
                    cwrate[peerIdx] = rate
    rates[i] = cwrate

for fidx in sorted(rates.keys()):
    for tidx in sorted(rates[fidx].keys()):
        print(fidx, tidx, rates[fidx][tidx])
