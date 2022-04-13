#!/usr/local/bin/python3

import argparse
import sys
import json

parser = argparse.ArgumentParser()
parser.add_argument("N", help="number of nodes", type=int)
parser.add_argument("D", help="degree", type=int)
args = parser.parse_args()

import networkx

out = {"Topology": []}

graph = networkx.generators.random_graphs.random_regular_graph(args.D, args.N)

for a, b in graph.edges():
    ne = {"From": a, "To": b}
    out["Topology"].append(ne)

print(json.dumps(out, sort_keys=True, indent=4))
