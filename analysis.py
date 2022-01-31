import math

def ripple(c, delta, k):
    return c * math.log(float(k)/delta) * math.sqrt(float(k))

def tau(c, delta, k, i):
    r = ripple(c, delta, k)
    th = int(round(float(k)/r))
    if i < th:
        return float(r) / float(i*k)
    elif i == th:
        log = math.log(r) - math.log(delta)
        r1 = r * log
        return r1 / float(k)
    else:
        return 0

def rho(k, i):
    if i == 1:
        return 1.0 / k
    else:
        return 1.0 / float(i*(i-1))

def robust_soliton(k, c, delta):
    pdf = [0.0 for i in range(k+1)]
    tot = 0.0
    for i in range(1, k+1):
        prob = rho(k, i) + tau(c, delta, k, i)
        pdf[i] = prob
        tot += prob
    for i in range(1, k+1):
        pdf[i] /= tot
    return pdf

# binomial probability with n trials, success prob p, and succ successes
def bp(n, p, succ):
    # algorithm copy pasted from https://www.johndcook.com/blog/2008/04/24/how-to-calculate-binomial-probabilities/
    temp = math.lgamma(n+1)
    temp -= math.lgamma(n-succ+1)
    temp -= math.lgamma(succ+1)
    temp += succ * math.log(p) + (n-succ) * math.log(1.0-p)
    return math.exp(temp)

# limit of throwing m balls into n bins when n=m*frac and m goes to inf
def frac_nonempty_bins_inf(frac):
    return 1.0 - math.exp(-1.0/frac)

# D: degree dist PMF
# decoded_tx_frac: fraction of transactions that have already been decoded
# cw_tx_ratio: ratio of number of codewords over transactions (1+overhead)
def iterative_peel(D, decoded_tx_frac, cw_tx_ratio):
    tx_cw_ratio = 1.0 / cw_tx_ratio
    newly_decoded_tx = 0.0
    newly_decoded_cw = 0.0
    maxDeg = len(D)-1
    while True:
        # decode all degree-1 codewords
        newly_decoded_cw = D[1]
        D[0] += newly_decoded_cw
        D[1] = 0.0
        # compute the fraction of currently-undecoded tx that just became decoded
        prob_undecoded_tx_decoded_now = frac_nonempty_bins_inf(tx_cw_ratio * (1.0-decoded_tx_frac) / newly_decoded_cw)
        newly_decoded_tx = prob_undecoded_tx_decoded_now * (1.0 - decoded_tx_frac)
        if prob_undecoded_tx_decoded_now < 0.000000000001:
            break
        # peel off newly-decoded txs from remaining codewords, and update array D
        for deg in range(maxDeg+1):
            prob = D[deg]
            D[deg] = 0.0    # we are going to examine all codewords for this degree
            for peeled in range(deg+1):
                D[deg-peeled] += prob * bp(deg, prob_undecoded_tx_decoded_now, peeled)
        # update decoded tx frac
        decoded_tx_frac += newly_decoded_tx
    return (D, decoded_tx_frac)


# This fn calculates the censorship probability

# There are two senders: alice, and bob. Alice is adversarial. She manages
# to know Alpha fraction of transactions the moment they show up, and pushes
# these transactions to us immediately, to mess up with the controller.

# Bob represents the honest party. They targets a Beta fraction of codeword
# loss rate.

# Calculate where the honest codeword rate should stabilize at
# K is the max degree

def calculate_delivery_rate(K, Alpha, Beta):
    pdf_remaining_degree = robust_soliton(K, 0.03, 0.5)
# calculate the remaining degree distribution after peeling the transactions obtained
# by the adversary
    for deg in range(1, K+1):
        prob = pdf_remaining_degree[deg]
        pdf_remaining_degree[deg] = 0.0
        for remaining in range(0, deg+1):
            to_peel = deg - remaining
            # calculate bernoully probability with to_peel successes and succ prob Alpha
            pdf_remaining_degree[remaining] += prob * bp(deg, Alpha, to_peel)
# find the smallest codeword rate that sustains the 2% loss given the distribution after
# peeling
    rate = 0.0
    d_fin = None
    tx_dec_frac = 0.0
    while True:
        rate += 0.01
        #print("trying", rate)
        d = [pdf_remaining_degree[i] for i in range(K+1)]
        (d_fin, tx_dec_frac) = iterative_peel(d, Alpha, rate)
        if d_fin[0] > 1.0-Beta:
            break
    return (rate, d_fin, tx_dec_frac)   # honest codeword rate, PMF of degree dist after decoding, frac of all transactions decoded

for k in [10, 20, 50, 70, 100]:
    r, d, t = calculate_delivery_rate(k, 0.000001, 0.02)
    print(k, t, r)

