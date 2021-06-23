#!/opt/homebrew/bin/gnuplot

set term pdf size 4.0,2.47 enhanced
set output "fig-iter-to-decode.pdf"
set xlabel "#tx decoded"
set ylabel "#codeword rcvd"
set notitle
set key top left

files = system("ls -1 *-mean-iter-to-decode.dat")

# get the prefix of a string ending (not incl.) at "-"
getTitle(s) = substr(s, 0, strstrt(s, "-")-1)

optimal(x) = x

plot for [file in files] file using 1:2 with lines title getTitle(file) lw 2, \
     optimal(x) with lines title "Optimal" lw 2 dt 2 lc 0
