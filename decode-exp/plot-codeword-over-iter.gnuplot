#!/opt/homebrew/bin/gnuplot

set term pdf size 4.0,2.47 enhanced
set output "fig-codeword-over-iter.pdf"
set xlabel "iteration"
set ylabel "#unreleased cw node0"
set notitle
set key top left

files = system("ls -1 *-p2-codeword-pool.dat")

# get the prefix of a string ending (not incl.) at "-"
getTitle(s) = substr(s, 0, strstrt(s, "-")-1)


plot for [file in files] file using 1:2 with lines title getTitle(file) lw 1.5
