/**
 * @file
 * @brief
 */

#ifndef BFD_REDIS_UTILS_H
#define BFD_REDIS_UTILS_H

#include <string>
#include <vector>
#include <sstream>
#include <sys/time.h>
#include <string.h>
#include <stdlib.h>

using namespace std;

static std::string int2string(int i)
{
	stringstream stream;
	stream << i;
	return stream.str();
}

static int string2int(string str)
{
	return atoi(str.c_str());
}

static vector<string> split(const std::string &s, char delim)
{
	vector<string> elems;

	std::stringstream ss(s);
	std::string item;
	while (std::getline(ss, item, delim))
	{
		elems.push_back(item);
	}
	return elems;
}

class TimeUtil
{
public:
	TimeUtil()
    {
        memset(&m_Start, 0, sizeof(m_Start));
        memset(&m_End, 0, sizeof(m_End));
    };

    void Start()
    {
        memset(&m_Start, 0, sizeof(m_Start));
    	gettimeofday(&m_Start, NULL);
    };

    /**
     * @brief
     * @param[in] bflag false ---毫秒
     * 					true  ---微秒
     */
    long End(bool bflag = false)
    {
        memset(&m_End, 0, sizeof(m_End));
        gettimeofday(&m_End, NULL);
        return getUseTime(bflag);
    };

    /**
     * @brief
     * @param[in] bflag false ---毫秒
     * 					true  ---微秒
     */
    long getUseTime(bool bflag = false)
    {
    	if (bflag)
    		return (1000000*(m_End.tv_sec-m_Start.tv_sec)
    	                               +(m_End.tv_usec-m_Start.tv_usec))/1000;
		else
			return 1000000*(m_End.tv_sec-m_Start.tv_sec)
    	                                          +(m_End.tv_usec-m_Start.tv_usec);
    }

    static string getCurrentTime()
    {
    	time_t now;
    	struct tm *timenow;

    	time(&now);
    	timenow = localtime(&now);

    	stringstream stream;
    	stream << timenow->tm_year + 1900 << "-"
    			<< timenow->tm_mon + 1 << "-"
    			<< timenow->tm_mday << " "
    			<< timenow->tm_hour + 1 << ":"
    			<< timenow->tm_min << ":"
    			<< timenow->tm_sec;

    	return stream.str();
    }
private:
    timeval m_Start;
    timeval m_End;

};



#endif
