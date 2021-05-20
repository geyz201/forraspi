strrand=$(head -c 3 /dev/random | base64)
testtime=$(date)
echo -e "${testtime} ${strrand}" >>time_rand.log
sudo nohup /home/pi/wristband/dsc 1>./MXRecord_${strrand}.txt 2>./MXtest_${strrand}.log &
nohup /home/pi/Serial/tty0 1>./ASRecord_${strrand}.txt 2>./AStest_${strrand}.log &

