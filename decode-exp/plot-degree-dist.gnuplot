#!/opt/homebrew/bin/gnuplot

set term pdf size 4.0,2.47 enhanced
set output "degree-dist.pdf"
set xlabel "#codeword degree"
set ylabel "count"
set notitle
set key top left

files = system("ls -1 *-codeword-degree-dist.dat")

# get the prefix of a string ending (not incl.) at "-"
getTitle(s) = substr(s, 0, strstrt(s, "-")-1)

plot for [file in files] file using 1:2:(1) with boxes title getTitle(file)
