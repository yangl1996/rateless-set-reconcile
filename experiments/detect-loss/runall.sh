rm *.csv
for i in {1..100}; do
	./detect-loss -k 500 -n $i -t 500 -m 5 -ntest 100 >> first.csv
done

for i in {1..100}; do
	./detect-loss -k 500 -n $i -t 500 -m 5 -ntest 100 -last >> last.csv
done
