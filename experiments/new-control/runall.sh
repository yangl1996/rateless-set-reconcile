for i in {1..100}; do
	./new-control -k 500 -n $i -t 500 -m 5 -ntest 100
done
