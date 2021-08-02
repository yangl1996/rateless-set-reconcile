set output "histogram.pdf"
set term pdf

binwidth=5
bin(x,width)=width*floor(x/width)

plot 'data.txt' using (bin($1,binwidth)):(1.0) smooth freq with boxes notitle
