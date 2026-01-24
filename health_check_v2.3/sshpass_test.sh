sshpass -p 'meralco' ssh -tt -o StrictHostKeyChecking=no meralco@172.10.1.1 << 'EOF'
terminal length 0
show version
exit
EOF

---------------------------------------

sshpass -p 'meralco' ssh -o StrictHostKeyChecking=no meralco@172.10.1.1 << 'EOF'
terminal length 0
show version
exit
EOF

---------------------------------

bash -c "sshpass -p 'meralco' ssh -tt -o StrictHostKeyChecking=no meralco@172.10.1.1 << 'EOF'
terminal length 0
show version
exit
EOF"

----------------------------------

echo -e "terminal length 0\nshow version\nexit" | sshpass -p 'meralco' ssh -tt -o StrictHostKeyChecking=no meralco@172.10.1.1

=---------------------------------

printf "terminal length 0\nshow version\nexit\n" | sshpass -p 'meralco' ssh -tt -o StrictHostKeyChecking=no meralco@172.10.1.1