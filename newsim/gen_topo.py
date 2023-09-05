#!python3

import argparse
import sys

parser = argparse.ArgumentParser()
parser.add_argument("N", help="number of nodes", type=int)
parser.add_argument("D", help="average degree", type=int)
parser.add_argument("--latency", help="use latency file", type=str, default="city-prop-delay.csv")
parser.add_argument("--topo", help="graph type", type=str, default="random-regular", choices=["random-regular", "random"])
args = parser.parse_args()

latency = {}
f = open(args.latency)
for line in f:
    if ',' in line:
        tokens = line.split(',')
        src = int(tokens[0])
        dst = int(tokens[1])
        delay = float(tokens[2])
        if not src in latency:
            latency[src] = {}
        latency[src][dst] = delay

# import the big library after parsing so ppl do not wait for the help message
import networkx
import numpy.random as nr

edgeTemplate = """{node1},{node2},{delay}"""

while True:
    if args.topo == "random-regular":
        graph = networkx.generators.random_graphs.random_regular_graph(args.D, args.N)
    elif args.topo == "random":
        graph = networkx.generators.random_graphs.gnm_random_graph(args.N, args.D*args.N/2)
    if networkx.is_connected(graph):
        break

for a, b in graph.edges():
    if a <= b:
        src = a
        dst = b
    else:
        src = b
        dst = a
    if src in latency and dst in latency[src]:
        l = int(latency[src][dst])
        if l < 1:
            l = 1
    else:
        l = 80
    print(edgeTemplate.format(node1=a, node2=b, delay=l))

