import math
# This file calculates the censorship probability

# There are two senders: alice, and bob. Alice is adversarial. She manages
# to know Alpha fraction of transactions the moment they show up, and pushes
# these transactions to us immediately, to mess up with the controller.

# Bob represents the honest party. They targets a Beta fraction of codeword
# loss rate.

# Calculate where the honest codeword rate should stabilize at
# K is the max degree

K=50
Alpha = 0.5
pdf_remaining_degree = [0.0 for i in range(K+1)]
Beta = 0.02

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

def iterative_peel(D, decoded_tx_frac, tx_cw_ratio):
    newly_decoded_tx = 0.0
    newly_decoded_cw = 0.0
    maxDeg = len(degree_dist)-1
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


# calculate the remaining degree distribution after peeling the transactions obtained
# by the adversary
for deg in range(1, K+1):
    if deg == 1:
        prob = 1.0 / K
    else:
        prob = 1.0 / float(deg) / float(deg-1)
    for remaining in range(0, deg+1):
        to_peel = deg - remaining
        # calculate bernoully probability with to_peel successes and succ prob Alpha
        pdf_remaining_degree[remaining] += prob * bp(deg, Alpha, to_peel)

# find the smallest codeword rate that sustains the 2% loss given the distribution after
# peeling


# assume the worst case that we lose transactions with the largest degree
# calculate CDF degree K until we reach Beta fraction


