rm *.csv
for i in {1..100}; do
	./detect-loss-overlap -k 500 -n $i -t 500 -m 5 -p 0 -ntest 100 >> first.csv
done

