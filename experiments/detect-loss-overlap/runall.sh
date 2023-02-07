rm *.csv
for i in {0..500}; do
	./detect-loss-overlap -k 500 -n 50 -t 500 -m 10 -p $i -ntest 100 >> first.csv
done

