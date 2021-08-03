set output "temp-cdf.pdf"
set term pdf

binwidth=5
bin(x,width)=width*floor(x/width)

plot 'data.txt' using (bin($1,binwidth)):(1.0) smooth cumulative with lines notitle
