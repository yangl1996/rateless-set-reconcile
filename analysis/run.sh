for t in 400 500 600
do
	echo $t
	for ld in {10..90} #0.1 0.2 0.3 0.4 0.5 0.6 0.7 0.8 0.9 
	do
		l="0.$ld"
		res=`./calc -l "$l" -f 0.01 -t $t 2> /dev/null | tail -1`
		tx=`echo $res | cut -d ' ' -f 6`
		cw=`echo $res | cut -d ' ' -f 4`
		echo "$l $t $tx $cw"
	done
	echo
	echo
done
