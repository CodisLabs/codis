#include"Reply.h"

using namespace bfd::codis;

Reply::Reply():error_(true),type_(ERROR),integer_(0) {
  // do nothing
}
Reply::Reply(redisReply *reply_p):type_(ERROR),integer_(0),error_(false) {
  type_ = static_cast<type_t>(reply_p->type);
  switch (type_) {
    case ERROR:
      error_ = true;
      str_ = std::string(reply_p->str, reply_p->len);
    case STRING:
    case STATUS:
      str_ = std::string(reply_p->str, reply_p->len);
      break;
    case INTEGER:
      integer_ = reply_p->integer;
      break;
    case ARRAY:
      for (size_t i=0; i < reply_p->elements; ++i) {
        elements_.push_back(Reply(reply_p->element[i]));
      }
      break;
    default:
      break;
  }
}
void Reply::SetErrorMessage(const string& message) {
  error_ = true;
  str_ = message;
}


