#include <iostream>
#include <time.h>
#include <fstream>
#include <pthread.h>
#include "BfdCodis.h"
#include "gtest/gtest.h"
#include "Utils.h"


using namespace std;
using namespace bfd::codis;
BfdCodis client1("192.168.168.130:2181", "/zk/codis/db_test0e/proxy", "item");
void *get_thread(void *parg) {
	client1.get("mykey0");
}
void read_from_file(){
	char buffer[2048];
	BfdCodis client("192.168.50.11:2181", "/zk/codis/db_item/proxy", "item");
    vector<string> keys;
    timeval start, end;
	ifstream in("keys.txt");
	while (!in.eof()) {
		in.getline(buffer, 1024);
		keys.push_back(buffer);	
	}		 
    gettimeofday(&start, NULL);
	int keys_len = keys.size();
	for (int j=0; j<100; j++) {
		for(int i=0; i<keys_len; ++i){
			client.get(keys[i]);
		}
	}
    gettimeofday(&end, NULL);
    cout <<"GET keys.txt 100 times, spend: "<< 1000*(end.tv_sec - start.tv_sec) +
        (end.tv_usec - start.tv_usec)/1000<< " ms. " <<endl;
    gettimeofday(&start, NULL);
	for (int i=0; i<100; ++i) {
		client.mget(keys);
	}
    gettimeofday(&end, NULL);
    cout <<"MGET keys.txt 100 tims, spend: "<< 1000*(end.tv_sec - start.tv_sec) +
        (end.tv_usec - start.tv_usec)/1000<< " ms. " <<endl;

} 
int main(int argc, char ** argv) {
	while (true) {
		vector<pthread_t> threads;
		for(int i=0; i<100; ++i) {
			pthread_t p;
			int i,ret;
			ret=pthread_create(&p,NULL,get_thread,NULL);
			if(ret!=0){
				printf ("Create pthread error!\n");	
				continue;
			}
			threads.push_back(p);	
		}
		for (int i=0; i<threads.size(); ++i) {
			pthread_join(threads[i],NULL);
		}
	}
    return 0;
}

