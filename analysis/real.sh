for t in 50 100 150 200 300 500
do
	echo $t
	for ld in {1..9}
	do
		l="0.$ld"
		../decode-exp/decode-exp -f "c($l)" -tc 3000 -l $t -p 4 -t 10000 -s 10000 -x 10000 &> /dev/null
		first=`sed -n '1000,1000p' out-mean-iter-to-decode.dat`
		last=`tail -n1 out-mean-iter-to-decode.dat`
		echo "$l $t $first $last"
	done
	echo
	echo
done
