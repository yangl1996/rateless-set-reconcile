#!/opt/homebrew/bin/gnuplot

set term pdf size 4.8,3
set output "d.pdf"
set xlabel "lookback window"
set ylabel "frac. decoded tx"
set key bottom left
set title "tx decodability under different codeword lookback windows"

set yrange [0:1]

plot for [i=0:*] "data-delay.txt" index i using 2:3 with lines lc i lw 1.7 notitle

