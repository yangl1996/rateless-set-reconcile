for t in 400 500 600
do
	echo $t
	for ld in {50..90}
	do
		l="0.$ld"
		../decode-exp/decode-exp -f "c($l)" -tc 30000 -l $t -t 100000 -s 1000 -x 1000 &> /dev/null
		first=`sed -n '1500,1500p' out-mean-iter-to-decode.dat`
		last=`tail -n1 out-mean-iter-to-decode.dat`
		echo "$l $t $first $last"
	done
	echo
	echo
done