#!/opt/homebrew/bin/gnuplot

set term pdf size 4.8,3
set output "p.pdf"
set xlabel "tx arrival rate / cw sending rate"
set ylabel "frac. decoded tx"
set key bottom left
set title "tx decodability under different codeword lookback windows (sim and model)"

set yrange [0:1]

plot for [i=0:*] "data-calc.txt" index i using 1:3 with lines lc i dt 2 lw 1.7 notitle, \
     for [i=0:*] "data-real.txt" index i using 1:(($5 - $3) / (($6 - $4) * $1)) with lines title columnheader(1) lc i lw 1.7, \
     NaN with lines dt 2 lw 1.7 lc 0 title "Model"

set title "cw decodability under different codeword lookback windows (model)"
plot for [i=0:*] "data-calc.txt" index i using 1:4 with lines lc i lw 1.7 title columnheader(1)
