#!/opt/homebrew/bin/gnuplot

set term pdf size 4.0,2.47 enhanced
set output "fig-ripple-dist.pdf"
set xlabel "ripple size"
set ylabel "fraction"
set notitle
set key top right
set logscale y

files = system("ls -1 *-ripple-size-dist.dat")

# get the prefix of a string ending (not incl.) at "-"
getTitle(s) = substr(s, 0, strstrt(s, "-")-1)

#set style fill transparent solid 0.3 noborder
plot for [file in files] file using 1:2 with linespoints title getTitle(file) lw 2 ps 0.7
