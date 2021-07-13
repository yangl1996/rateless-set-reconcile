#!/opt/homebrew/bin/gnuplot

set xlabel "iteration"
set ylabel "#tx unique to p1"
set notitle
set key top left

files = system("ls -1 *-ntx-unique-to-p1.dat")

# get the prefix of a string ending (not incl.) at "-"
getTitle(s) = substr(s, 0, strstrt(s, "-")-1)

allset = 0
bind all 'x' 'allset = 1'

while (!allset) {
	plot for [file in files] file using 1:2 with lines title getTitle(file) lw 1.5
	pause 0.2
}
