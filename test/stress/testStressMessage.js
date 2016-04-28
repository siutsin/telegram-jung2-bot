'use strict';

const MessageController = require('../../controller/message');
const mongoose = require('mongoose');
const Message = require('../../model/message');
const co = require('co');
const faker = require('faker');

const sample = {
  chat: {
    id: 'stubChatId'
  }
};

function repeat(n, recreatePromise){
  return co(function *(){
    for(var i=0; i<n; i++) yield recreatePromise();
  })
}

describe('MessageStressTest', function () {
  this.timeout(60*60*1000);

  before(function(done){
    // connect to local database for testing
    var connectionString = '127.0.0.1:27017/jung2botTest';
    mongoose.connect(connectionString, {db: {nativeParser: true}});

    Message.count({}).then( n => {
      if( n < 1000 ) done(new Error('please populate database before testing'));
      else done();
    });
  });

  describe('getTopTen', function(){

    it('test 1: once', function(done){
      MessageController.getTopTen(sample, true).then(function(msg){
        //console.log(msg);
        done();
      }, e => console.error(e.stack));
    });

    it('test 2: repeat 10 times', function(done){
      repeat(10, () => MessageController.getTopTen(sample, true)).then(done);
    });

  });

  describe('getAllJung', function(){

    it('test 1: once', function(done){
      MessageController.getAllJung(sample, true).then(function(msg){
        //console.log(msg);
        done();
      })
    });

    it('test 2: repeat 10 times', function(done){
      repeat(10, () => MessageController.getAllJung(sample, true)).then(done);
    });

  })


});