#include"Command.h"
using namespace std;
using namespace bfd::codis;

Command::Command(const std::string& arg)
{
	args_.push_back(arg);
}

Command& Command::operator()(const char* arg)
{
	args_.push_back(string(arg));
	return *this;
}

Command& Command::operator()(const std::string arg)
{
	args_.push_back(arg);
	return *this;
}

std::string Command::ToString()
{
	std::stringstream ss;
	for (size_t i = 0; i < args_.size(); ++i)
	{
		ss << args_[i];
		if (i != args_.size() - 1)
		{
			ss << " ";
		}
	}
	return ss.str();
}

