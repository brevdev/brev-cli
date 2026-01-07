export BREV_USER=brevcloud
id -u $BREV_USER >/dev/null 2>&1 || sudo useradd -m -s /bin/bash $BREV_USER
sudo chmod 0700 /home/$BREV_USER
export BREV_HOME=/home/$BREV_USER
echo "$BREV_USER ALL=(ALL) NOPASSWD:ALL" | sudo tee /etc/sudoers.d/$BREV_USER >/dev/null
sudo chmod 0440 /etc/sudoers.d/$BREV_USER
sudo visudo -c -f /etc/sudoers.d/$BREV_USER
sudo install -d -m 700 -o $BREV_USER -g $BREV_USER $BREV_HOME/.ssh
sudo touch $BREV_HOME/.ssh/authorized_keys
sudo chmod 600 $BREV_HOME/.ssh/authorized_keys
sudo mkdir $BREV_HOME/.ssh
sudo touch $BREV_HOME/.ssh/authorized_keys
sudo chmod 700 $BREV_HOME/.ssh
sudo chmod 600 $BREV_HOME/.ssh/authorized_keys
sudo chown -R $BREV_USER:$BREV_USER /home/$BREV_USER/.ssh
export BREV_KEY='<PUB KEY>'
sudo grep -qxF "$BREV_KEY" $BREV_HOME/.ssh/authorized_keys ||   echo "$BREV_KEY" | sudo tee -a $BREV_HOME/.ssh/authorized_keys >/dev/null
cat /home/$BREV_USER/.ssh/authorized_keys
sudo cat /home/$BREV_USER/.ssh/authorized_keys
sudo chown -R $BREV_USER:$BREV_USER $BREV_HOME
sudo -u $BREV_USER whoami
sudo -u $BREV_USER ls -la $BREV_HOME