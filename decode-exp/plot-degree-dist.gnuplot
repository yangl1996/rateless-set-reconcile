#!/opt/homebrew/bin/gnuplot

set term pdf size 4.0,2.47 enhanced
set output "degree-dist.pdf"
set xlabel "codeword degree"
set ylabel "fraction"
set notitle
set key top right

files = system("ls -1 *-codeword-degree-dist.dat")

soliton(x) = x < 2 ? NaN : 1.0 / (x * (x-1.0))

# get the prefix of a string ending (not incl.) at "-"
getTitle(s) = substr(s, 0, strstrt(s, "-")-1)

#set style fill transparent solid 0.3 noborder
plot for [file in files] file using 1:2 with linespoints title getTitle(file) lw 2 ps 0.7, \
    soliton(x) with lines title "Ideal Soliton" lw 2 dt 3 lc 0
