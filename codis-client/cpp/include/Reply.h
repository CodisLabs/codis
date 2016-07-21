#ifndef REPLY_H_
#define REPLY_H_
#include <iostream>
#include "hiredis.h"
#include <vector>
using namespace std;

namespace bfd
{
namespace codis
{
class Reply
{
public:
  Reply();
  Reply(redisReply* reply);
  enum type_t
  {
    STRING = 1,
    ARRAY = 2,
    INTEGER = 3,
    NIL = 4,
    STATUS = 5,
    ERROR = 6
  };
  void SetErrorMessage(const string& message);
  inline type_t type() const {return type_;}
  inline const string str() const {return str_;}
  inline long long integer() const {return integer_;}
  inline const vector<Reply>& elements() const {return elements_;}
  inline const bool error() const {return error_;}
private:
  bool error_;
  type_t type_;
  string str_;
  long long integer_;
  std::vector<Reply> elements_;

};
}
}
#endif
