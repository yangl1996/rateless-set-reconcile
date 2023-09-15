for i in 2 4 6 8 10 12 14 16 18 20 30 40 50; do
	res=`cpuset -l 0 ./benchmark -s 1000000 -d $i -n 100`
	echo $i $res
done

 
for i in 60 70 80 90 100 200 300 400 500; do
	res=`cpuset -l 0 ./benchmark -s 1000000 -d $i -n 60`
	echo $i $res
done
       
for i in 600 700 800 900 1000 2000 4000; do
	res=`cpuset -l 0 ./benchmark -s 1000000 -d $i -n 30`
	echo $i $res
done
       
for i in 6000 8000 10000 20000 40000 60000 100000; do
	res=`cpuset -l 0 ./benchmark -s 1000000 -d $i -n 10`
	echo $i $res
done
