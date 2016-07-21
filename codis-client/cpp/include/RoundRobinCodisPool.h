/**
 * @file
 * @brief
 */

#ifndef CODIS_CLIENT_ROUNDROBINCODISPOOL_H
#define CODIS_CLIENT_ROUNDROBINCODISPOOL_H

#include "CodisClient.h"
#include <vector>
#include <zookeeper.h>

using namespace std;

namespace bfd
{
namespace codis
{

class RoundRobinCodisPool
{
public:
	RoundRobinCodisPool(const string& zookeeperAddr, const string& proxyPath, const string& businessID);
	~RoundRobinCodisPool();

	CodisClient* GetProxy();
private:
	vector<CodisClient*> m_Proxys;
	int proxyIndex;
	vector<pair<string, int> > m_ProxyInfos;
	zhandle_t *m_Zh;
	string m_ZookeeperAddr;
	string m_ProxyPath;
	string m_BusinessID;
	pthread_mutex_t m_Mutex;
private:
	void Init(zhandle_t *(&zh), const string& proxyPath);
	static void proxy_watcher(zhandle_t *zh, int type, int state, const char *path, void *context);
	string ZkGet(zhandle_t *(&zh), const string &path, bool watch=true);
	vector<pair<string, int> > GetProxyInfos(zhandle_t *(&zh), const string& proxyPath);
	void InitProxyConns(vector<pair<string, int> >& proxyInfos);

};

}
}
#endif
