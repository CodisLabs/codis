CXX = g++
OBJ_DIR = obj
TEST_DIR = test

INCPATH = -Iinclude
INCPATH += -I3party/include
INCPATH += -I3party/include/hiredis
INCPATH += -I3party/include/zookeeper

LFLAGS += -L3party -lpthread
LFLAGS += -Lbin -lcodisclient
LFLAGS += -Lbin -ljson

TARGET = bin/CodisClientDemo

CXX_OBJS = $(OBJ_DIR)/CodisClientDemo.o 
			
$(OBJ_DIR)/%.o:$(TEST_DIR)/%.cpp
	$(CXX) -c -fPIC -o $@ $< $(INCPATH) $(LFLAGS)
	
$(OBJ_DIR)/%.o:$(TEST_DIR)/%.c
	$(CXX) -c -fPIC -o $@ $< $(INCPATH) $(LFLAGS)

			
$(TARGET):$(CXX_OBJS)
	$(CXX) -o $(TARGET) $(CXX_OBJS) $(LFLAGS)
	
.PHONY:clean
clean:
	-rm -f bin/RedisClientTest
