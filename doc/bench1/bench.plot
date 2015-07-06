set terminal pdfcairo enhanced color lw 1.2 size 20cm,34cm
set output 'bench.pdf'

set border 11 back

set tics font ",14"
set key spacing 1.8 samplen 2 font ",15" left Left reverse
set key box opaque

set grid y
set xtics nomirror out
set ytics nomirror out
set y2tics nomirror out
set mytics 5
set my2tics 5
set boxwidth 0.8 relative

set pointsize 0.8

set style line 1 lc rgb "#4873EA" lw 1.4 pt 7
set style line 2 lc rgb "#2FA75A" lw 1.4 pt 7
set style line 3 lc rgb "#EB383E" lw 1.4 pt 7

set offset -0.3,-0.3,0,0

set xtic add ("" 0, "1" 1, "2" 2, "4" 3, "8" 4, "16" 5, "32" 6, "64" 7, "100" 8, "200" 9, "300" 10, "400" 11, "500" 12, "800" 13, "" 14)

set y2label "99% Response Time (ms)" offset 1,0 font ",16"

set lmargin at screen 0.10
set rmargin at screen 0.90

set multiplot layout 3,1

set ylabel "RPS_{avg}=(RPS_{set}+RPS_{get})/2, 1K=10^3" offset 0.5,0 font ",16"
set xlabel "Number of Clients" offset 0,-0.5 font ",16"

set bmargin at screen 0.72
set tmargin at screen 0.96

set title "RedisBenchmark(SET+GET): DataSize=256B, Pipeline=Disabled" font ",20"
set ytics 50000
set ytic add ("50K" 50000, "100K" 100000, "150K" 150000, "200K" 200000)
set y2range[0:8]

plot 'input1' \
              using 1:2 title "1Core-Twemproxy " with linespoints ls 1, \
           '' using 1:4 title "4Core-Codis 2.0 " with linespoints ls 3, \
           '' using 1:5 notitle with points ls 1 pt 1 ps 0.7 axes x1y2, \
           '' using 1:7 notitle with points ls 3 pt 1 ps 0.7 axes x1y2, \
           '' using 1:3 title "4Core-Codis 1.9 " with linespoints ls 2, \
           '' using 1:6 notitle with points ls 2 pt 1 ps 0.7 axes x1y2, \


set bmargin at screen 0.39
set tmargin at screen 0.63

set title "RedisBenchmark(SET+GET): DataSize=256B, Pipeline=100" font ",20"
set ytics 200000
set ytic add ("200K" 200000, "400K" 400000, "600K" 600000, "800K" 800000, "1000K" 1000000)
set yrange[0:1000000]
set y2range[0:350]

plot 'input2' \
              using 1:2 title "1Core-Twemproxy " with linespoints ls 1, \
           '' using 1:4 title "4Core-Codis 2.0 " with linespoints ls 3, \
           '' using 1:5 notitle with points ls 1 pt 1 ps 0.7 axes x1y2, \
           '' using 1:7 notitle with points ls 3 pt 1 ps 0.7 axes x1y2, \
           '' using 1:3 title "4Core-Codis 1.9 " with linespoints ls 2, \
           '' using 1:6 notitle with points ls 2 pt 1 ps 0.7 axes x1y2, \

set bmargin at screen 0.06
set tmargin at screen 0.30

set title "RedisBenchmark(MSET): DataSize=256B, Pipeline=100" font ",20"
set ytics 50000
set ytic add ("50K" 50000, "100K" 100000, "150K" 150000, "200K" 200000)
set yrange[0:150000]
set y2range[0:2000]

set ylabel "RPS_{mset}, 1K=10^3" offset 0.5,0 font ",16"

plot 'input3' \
              using 1:2 title "1Core-Twemproxy " with linespoints ls 1, \
           '' using 1:4 title "4Core-Codis 2.0 " with linespoints ls 3, \
           '' using 1:5 notitle with points ls 1 pt 1 ps 0.7 axes x1y2, \
           '' using 1:7 notitle with points ls 3 pt 1 ps 0.7 axes x1y2, \
           '' using 1:3 title "4Core-Codis 1.9 " with linespoints ls 2, \
           '' using 1:6 notitle with points ls 2 pt 1 ps 0.7 axes x1y2, \

unset multiplot
