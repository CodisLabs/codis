/**
 * @file
 * @brief
 */

#ifndef BFD_REDIS_SCOPEDLOCK_H
#define BFD_REDIS_SCOPEDLOCK_H
using namespace std;
class ScopedLock
{
public:
	ScopedLock(pthread_mutex_t &mutex)
	{
		m_Mutex = &mutex;
		pthread_mutex_lock(m_Mutex);
	}

	~ScopedLock()
	{
		pthread_mutex_unlock(m_Mutex);
	}


private:
	pthread_mutex_t *m_Mutex;
};

#endif
