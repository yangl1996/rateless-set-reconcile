for t in 400 500 600
do
	for ld in {10..90} #0.1 0.2 0.3 0.4 0.5 0.6 0.7 0.8 0.9 
	do
		l="0.$ld"
		res=`./calc -l "$l" -f 0.01 -t $t 2> /dev/null | tail -1 | cut -d ' ' -f6`
		echo "$l $t $res"
	done
	echo
	echo
done
