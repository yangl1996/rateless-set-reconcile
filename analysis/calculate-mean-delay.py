f = open("data-delay.txt", "r")
mean = 0.0
lastFrac = 0.0
delay = 0
for line in f:
    tokens = line.strip().split(' ')
    delay = int(tokens[1])
    frac = float(tokens[2])
    if frac >= lastFrac:
        mean += float(delay) * (frac - lastFrac)
        lastFrac = frac
    else:
        break
print("max delay", delay)
print(mean)
