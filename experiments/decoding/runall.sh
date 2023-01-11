for i in {50..100}; do
	res=`./simple -k 50 -n $i -t 50 -m 0 -ntest 1500 | cut -f3 -d' '`
	printf "%d %.2f\n" $i $res
done
