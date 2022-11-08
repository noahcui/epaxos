for idx in 1 2 3 4 5
do
bin/client -e -T $idx -think 1000 > $1
done
