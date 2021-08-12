#include "stdio.h"
#include "stdlib.h"
#include "unistd.h"
#include "string.h"
#include "math.h"

long double l;	// transaction arrival rate normalized to codeword rate
long double f;	// codeword filter probability
int MaxT;	// max number of timeslots to look back when building codewords
int MaxDeg;	// max degree that we consider, equals to MaxT * l

long double* D;	// Fraction of codewords that is of degree i
long double* nextD;	// Used as the staging area for the next estimation of D
long double Decoded;		// P(tx T is decoded)

// Binomial probability with n trials, x successes, and success
// probability p.
long double binom_prob(int n, int x, long double p);

// Fraction of nonempty bins if we throw m balls uniformly into n bins.
long double frac_nonempty_bins(int m, int n);

// Limit of frac_nonempty_bins when n=frac * m and m goes to +inf.
long double frac_nonempty_bins_inf(long double frac);

// Try decode more codewords. Returns 1 if more is decoded; 0 if the process
// is stuck.
int visit_deg();

int main(int argc, char *argv[]) {
	int opt;
	while ((opt = getopt(argc, argv, "t:f:l:")) != -1) {
		switch (opt) {
			case 't':
				MaxT = atoi(optarg);
				break;
			case 'f':
				f = atof(optarg);
				break;
			case 'l':
				l = atof(optarg);
				break;
		}
	}
	// init the numbers
	MaxDeg = l * MaxT;
	D = (long double*)calloc(MaxDeg+1, sizeof(long double));
	nextD = (long double*)calloc(MaxDeg+1, sizeof(long double));
	Decoded = 0.0;
	for (int i = 0; i <= MaxDeg; i++) {
		D[i] = binom_prob(MaxT, i, f * l);
	}

	int iter = 0;
	while (visit_deg()) {
		printf("[iter %d] cw %.17Lg, tx %.17Lg\n", iter, D[0], Decoded);
		iter += 1;
	}
	printf("[final] decodable cw %.17Lg tx %.17Lg\n", D[0], Decoded);

	return 0;
}

int visit_deg() {
	// decode all degree-1 codewords
	long double newly_decoded_cw = D[1];
	D[0] += newly_decoded_cw;
	D[1] = 0;
	// the fraction of currently-undecoded tx that just became decoded
	long double convert_tx_frac = frac_nonempty_bins_inf(l * (1.0-Decoded) / newly_decoded_cw);
	// the fraction of all tx that just became decoded
	long double newly_decoded_tx = convert_tx_frac * (1.0 - Decoded);

	if (convert_tx_frac < 0.000000000001) {
		return 0;
	}
	// peel off newly decoded txs from remaining codewords, and calculate
	// a new array D
	// NOTE: we are basically doing matrix multiplication
	memset(nextD, 0, (MaxDeg+1) * sizeof(long double));
	for (int i = 0; i <= MaxDeg; i++) {
		// for each degree-i codeword, compute the distribution of the
		// number of transactions that can be newly peeled
		for (int np = 0; np <= i; np++) {
			// probability that among the i txs remaining in the
			// codeword, np of them are decoded
			long double prob_np = binom_prob(i, np, convert_tx_frac);
			nextD[i-np] += D[i] * prob_np;
		}
	}
	// update D and Decoded
	memcpy(D, nextD, (MaxDeg+1) * sizeof(long double));
	Decoded = Decoded + newly_decoded_tx;
	return 1;
}

long double binom_prob(int n, int x, long double p) {
	// algorithm copy pasted from https://www.johndcook.com/blog/2008/04/24/how-to-calculate-binomial-probabilities/
	long double temp = lgamma(n + 1);
	temp -= lgamma(n - x + 1);
	temp -= lgamma(x + 1);
	temp += x * log(p) + (n-x) * log(1.0-p);
	return exp(temp);
}

long double frac_nonempty_bins(int m, int n) {
	long double res = 1.0 - pow(1.0 - 1.0 / n, m);
	return res;
}

long double frac_nonempty_bins_inf(long double frac) {
	return 1.0 - exp(-1.0 / frac);
}
