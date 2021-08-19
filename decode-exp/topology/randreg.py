import networkx
import json


n = networkx.random_regular_graph(10, 101)
c = {
        "MirrorProb": 0,
        "Seed": 0,
        "TimeoutDuration": 500,
        "TimeoutCounter": 0,
        "DegreeDist": "u(0.01)",
        "LookbackTime": 0,
        "ParallelRuns": 1,
        "Servers": [],
        "InitialCommonTx": 0,
        "Connections": []
}

for node in n.nodes():
    c["Servers"].append({
        "Name": str(node),
        "InitialUniqueTx": 0,
        "TxArrivePattern": "p(0.07)"
        })

for edge in n.edges():
    c["Connections"].append({
        "Car": str(edge[0]),
        "Cdr": str(edge[1])})


print(json.dumps(c, indent=4))
