#!/usr/local/bin/gnuplot

set term pdf size 3.8,1.4 #font ",13"
set output "compare-metrics.pdf"
set multiplot layout 1,3
set notitle

f(r, d) = (r + 1.0) * (1.0 + d * 4.0 / 128.0) - 1.0
g(r, d) = r * 19.0 / 18.0 * (1.0 + d * 4.0 / 128.0) - 1.0

set lmargin 5
set rmargin 2.1
set bmargin 3.2
set tmargin 1.8

set xrange [0:4000]
set xtics ("0" 0, "2000" 2000, "4000" 4000)

set ylabel "Latency (s)" offset 1,0
set xlabel "Transaction Rate"
set yrange [0:2]
set ytics 1
plot "real.csv" using 1:9:8:10 notitle with errorlines ps 0.7 lw 1.5 lc rgb '#4dbeee', \
"sim.csv" using 1:9:8:10 notitle with errorlines ps 0.7 lw 1.5 lc rgb '#77ac30'

set ylabel "Overhead" offset 1.6,0
set xlabel "Transaction rate"
set yrange [0:1]
set ytics 0.5
set key tmargin center  maxrows 1
plot "real.csv" using 1:(g($6, 5)):(g($5, 5)):(g($7, 5)) title "Real-world" with errorlines ps 0.7 lw 1.5 lc rgb '#4dbeee', \
"sim.csv" using 1:(f($6, 5)):(f($5, 5)):(f($7, 5)) title "Simulation" with errorlines ps 0.7 lw 1.5 lc rgb '#77ac30'

set ylabel "Delivery rate"
set xlabel "Transaction rate"
set yrange [0:1]
plot "real.csv" using 1:3:2:4 notitle with errorlines ps 0.7 lw 1.5 lc rgb '#4dbeee', \
"sim.csv" using 1:3:2:4 notitle with errorlines ps 0.7 lw 1.5 lc rgb '#77ac30'
