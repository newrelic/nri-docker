# Virtual machine setup to run the integration

This is a step by step guide to setup a machine to run the integration locally in a virtual machine,
so making prof-of-concept changes gets easier.

## Set up the virtual machine

We are using [Vagrant](https://www.vagrantup.com/intro) as an example. The Vagrantfile bellow may be used to both,
meet the [usage requirements](https://docs.newrelic.com/docs/infrastructure/install-infrastructure-agent/linux-installation/docker-instrumentation-infrastructure-monitoring/#requirements) and meet the building requirements.

```ruby
# -*- mode: ruby -*-
Vagrant.configure("2") do |config|
  config.vm.box = "bento/ubuntu-21.10"
  # Sync the repository folder in the virtual machine
  config.vm.synced_folder "<absolute-path-to-repository-folder>", "/code"
  # Installs both docker and golang 1.8
  config.vm.provision "shell", inline: <<-SHELL
     apt-get update
     apt-get install \
      ca-certificates \
      curl \
      gnupg \
      lsb-release
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
    echo \
      "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu \
      $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
    apt-get update
   apt-get install -y docker-ce docker-ce-cli containerd.io
   cd /tmp
   wget https://go.dev/dl/go1.18.2.linux-amd64.tar.gz
   rm -rf /usr/local/go && tar -C /usr/local -xzf go1.18.2.linux-amd64.tar.gz
   echo "export PATH=$PATH:/usr/local/go/bin" >> /etc/profile
   SHELL
end
```

Then, you can start the corresponding virtual machine and connect to it:

```
$ vagrant up
$ vagrant ssh
```

## Run the the integration tests

```
vagrant@vagrant$ sudo -i
root@vagrant# cd /code
root@vagrant# make integration-test
```

## Setup the integration and gather some data into your account

First, [Install the infrastructure agent](https://docs.newrelic.com/docs/infrastructure/install-infrastructure-agent/linux-installation/install-infrastructure-monitoring-agent-linux/).

Then, compile and install the integration:

```
root@vagrant# cd /code
root@vagrant# make
root@vagrant# make install
```

Check if nri-docker integration is configured. Eg:

```
root@vagrant# cat /etc/newrelic-infra/integrations.d/docker-config.yml
integrations:
  - name: nri-docker
    feature: docker_enabled
    interval: 15s
```

Finally, [Restart the infrastructure agent](https://docs.newrelic.com/docs/infrastructure/install-infrastructure-agent/manage-your-agent/start-stop-restart-infrastructure-agent/).


You may launch some containers to get data from, in a similar way it is done in integration tests:

```
root@vagrant# cd /code
root@vagrant# docker build -t stress ./src/biz
root@vagrant# docker run --rm -it stress stress-ng -c 2 -l 95 --io 3 -t 5m
```
