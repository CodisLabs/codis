# Codis & Pika guide(sharding mode)

## Prepare

### Before start, learn about [Codis](https://github.com/CodisLabs/codis) and [Pika](https://github.com/OpenAtomFoundation/pika)

### Build codis image 
```
cd codis_dir
docker build -f Dockerfile -t codis-image:v3.2 .
```

### Pika configure
Make sure **instance-mode** in pika.conf is set to sharding and **default-slot-num** should be 1024. Cause Codis's slot number is 1024. According to your situation, you can change **default-slot-num** by rebuilding codis code.

```
instance-mode : sharding
default-slot-num : 1024
```
---
## Start 

### 1. Make it run
- Run the following orders one by one
```
cd codis_dir/scripts
sudo bash docker_pika.sh zookeeper
sudo bash docker_pika.sh dashboard
sudo bash docker_pika.sh proxy
sudo bash docker_pika.sh fe
sudo bash docker_pika.sh pika 
```

- Or run the orders in one line with some sleep 
``` 
sudo bash docker_pika.sh zookeeper && sleep 10s && sudo bash docker_pika.sh dashboard && sudo bash docker_pika.sh proxy && sudo bash docker_pika.sh fe && sudo bash docker_pika.sh pika
```

### 2. Configure codis by fe end
#### Configure clusetr in codis fe（ http://fehost:8080/ ）
- make proxy
<img width="1263" alt="image" src="https://user-images.githubusercontent.com/6240382/168002719-81d98b88-c818-4d60-8c17-793861258314.png">
- make group
<img width="1261" alt="image" src="https://user-images.githubusercontent.com/6240382/168003257-f8267cd8-5c86-4f7f-9389-c100f1d047ea.png">
- rebalance slot 
<img width="1242" alt="image" src="https://user-images.githubusercontent.com/6240382/168003051-576e6dc9-d6f4-496d-8c73-4c094d599ed8.png">


### 2. Configure codis by command
- log in dashboard instance and use dashboard-admin to make operations above

<img width="701" alt="image" src="https://user-images.githubusercontent.com/6240382/168003825-d6dd180b-05f3-4d05-a6ae-9f4497753903.png">



### 3. Init pika
- add slot for pika

Supposing that you have 4 groups, Pika instancse should be assinged to 4 groups. Every Pika instance should make 256 slots.(1024/4=256) Offset and end depend on which group it's in.
```
# pika in group 1
pkcluster addslots 0-255
# pika in group 2
pkcluster addslots 256-511
# pika in group 3
pkcluster addslots 512-767
# pika in group 4
pkcluster addslots 768-1023
```

### 4. Test proxy
- Connenct client by Codis proxy and test it.

<img width="297" alt="image" src="https://user-images.githubusercontent.com/6240382/168005210-59906125-8c0a-4409-a832-b632d8632336.png">

## DevOps

### Slave of a master
```
pkcluster slotsslaveof masterIp masterPort slotOffset-slotEnd
```

### Migrate group 1 to group 5
- 1. create new group 5 
- 2. Make group 5 master instance be slave of group 1 master instance
- 3. Make group 5 slave instances be slave of group 5 master instance
- 4. When lag between group 1 master and group 5 master is small, make all group 1 slot to group 5.
- 5. When lag between group 1 master and group 5 master is 0, make group 5 master instance slave of no one. 
- 6. delete group 1 instances.


