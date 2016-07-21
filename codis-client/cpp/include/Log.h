/**
 * @file
 * @brief
 */

#ifndef REDIS_SENTINEL_CLIENT_LOGGER_H
#define REDIS_SENTINEL_CLIENT_LOGGER_H

#include <fstream>
#include <unistd.h>
#include "Utils.h"


#define LOG(level, msg) Log(level, msg, __FILE__, __LINE__)

enum LEVEL{INFO, WARN, ERROR};

static pthread_mutex_t* GetMutex()
{
	static pthread_mutex_t logmutex = PTHREAD_MUTEX_INITIALIZER;

	return &logmutex;
}

static void Log(LEVEL level, string msg, string file, int line)
{
	pthread_mutex_lock(GetMutex());

	int pid = (int)getpid();

	stringstream stream;
	stream << "codis_client_log_" << pid << ".txt";

	ofstream ofs(stream.str().c_str(), ios::app);
	if (!ofs)
	{
		pthread_mutex_unlock(GetMutex());
		return;
	}

	ofs << "[";
	switch (level)
	{
	case INFO:
		ofs << "INFO" << "]";
		break;
	case WARN:
		ofs << "WARN" << "]";
		break;
	case ERROR:
		ofs << "ERROR" << "]";
		break;
	default:
		ofs << "ERROR" << "]";
	};

	ofs << "[" << TimeUtil::getCurrentTime() << "]";

	ofs << "[" << file << ":" << line << "]";

	ofs << "[" << msg << "]" << endl;

	ofs.close();

	pthread_mutex_unlock(GetMutex());
}

#endif
