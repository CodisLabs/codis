/**
 * @file
 * @brief
 */

#ifndef BFD_SENTINEL_CLIENT_COMMAND_H
#define BFD_SENTINEL_CLIENT_COMMAND_H

#include <iostream>
#include <vector>
#include <string>
#include <sstream>

namespace bfd
{
namespace codis
{

class Command
{
public:
	Command(){};

	Command(const std::string& arg);

	~Command(){};

	Command& operator<<(const std::string& arg);

	Command& operator()(const char* arg);

	Command& operator()(const std::string arg);

	operator const std::vector<std::string>&()
	{
		return args_;
	};

	std::string ToString();

	std::vector<std::string> args() const
	{
		return args_;
	};
private:
	std::vector<std::string> args_;
};
}
}
#endif
