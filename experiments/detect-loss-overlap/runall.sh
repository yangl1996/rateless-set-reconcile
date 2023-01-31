rm *.csv
for i in {0..490}; do
	./detect-loss-overlap -k 500 -n 50 -t 500 -m 5 -p $i -ntest 100 >> first.csv
done

