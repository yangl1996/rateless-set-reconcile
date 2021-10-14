import networkx
import json


n = networkx.random_regular_graph(10, 101)
c = {
        "Seed": 0,
        "Servers": [],
        "Connections": [],
        "Sources": [],
}

for node in n.nodes():
    c["Servers"].append({
        "Name": str(node),
        "TimeoutDuration": 500,
        "TimeoutCounter": 0,
        "DegreeDist": "u(0.01)",
        "LookbackTime": 300
        })
    c["Sources"].append({
        "Name": str(node),
        "ArrivePattern": "p(0.07)",
        "InitialTx": 0,
        "Targets": [str(node)]
        })

for edge in n.edges():
    c["Connections"].append({
        "Car": str(edge[0]),
        "Cdr": str(edge[1])})


print(json.dumps(c, indent=4))
