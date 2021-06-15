#!/opt/homebrew/bin/gnuplot

set term pdf size 4.0,2.47 enhanced
set output "iter-to-decode.pdf"
set xlabel "#tx decoded"
set ylabel "#codeword rcvd"
set notitle

plot "out-mean-iter-to-decode.dat" using 1:2 with lines title 1 lw 2
