#!/bin/sh
[ -r /etc/lsb-release ] && . /etc/lsb-release

if [ -z "$DISTRIB_DESCRIPTION" ] && [ -x /usr/bin/lsb_release ]; then
	# Fall back to using the very slow lsb_release utility
	DISTRIB_DESCRIPTION=$(lsb_release -s -d)
fi

printf "                               _  
"
printf "              _        ,-.    / ) 
"
printf "             ( \`.     // /-._/ / 
"
printf "              \`\ \   /(_/ / / /  
"
printf "                ; \`-\`  (_/ / /  
"
printf "                |       (_/ /     
"
printf "                \          /      
"
printf "Welcome to       )       /\`      
"
printf "      brev.dev   /      /\`       
"
printf "

Internet Speed:
"

    printf "Avg Upload: 1574.31 Mbit/s
"
printf "Avg Download: 1103.68 Mbit/s
"
printf "
Running %s (%s %s %s)
" "$DISTRIB_DESCRIPTION" "$(uname -o)" "$(uname -r)" "$(uname -m)"
    