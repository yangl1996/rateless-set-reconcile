for t in {1..600}
do
	l="0.70"
	res=`./calc -l "$l" -f 0.01 -t $t 2> /dev/null | tail -1`
	tx=`echo $res | cut -d ' ' -f 6`
	cw=`echo $res | cut -d ' ' -f 4`
	echo "$l $t $tx $cw"
done
