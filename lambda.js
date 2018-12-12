/* eslint-disable  func-names */
/* eslint-disable  no-console */

const Alexa = require('ask-sdk-core');
var   recipe = {task:[] }
var   taskNum = 0
// Options to be used by request 

const LaunchRequestHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'LaunchRequest';
  },
  
  handle(handlerInput) {
    var speechText = '';
    const http = require('https');
    var params = {
      host: '5h6o821oqg.execute-api.us-east-1.amazonaws.com',
      port: '443',
      path: '/Prod/next/OOO?sid='+handlerInput.requestEnvelope.session.sessionId
    };
    // this is a get, so there's no post data
    //params.path=path+handlerInput.requestEnvelope.session.sessionId
    promise = new Promise((resolve, reject) => {
        var req = http.request(params, function(res) {
            // reject on bad status
            if (res.statusCode < 200 || res.statusCode >= 300) {
                return reject(new Error('statusCode=' + res.statusCode));
            }
            // cumulate data
            var body = [];
            res.on('data', function(chunk) {
                body.push(chunk);
                recipe = JSON.parse(body)
                speechText=recipe.task[0]
            });
            // resolve on end
            res.on('end', function() {
                try {
                    body = body;
                } catch(e) {
                    reject(e);
                }
                resolve(body);
            });
        });
        // reject on request error
        req.on('error', function(err) {
            // This is not a "Second reject", just a different sort of failure
            reject(err);
        });
        // IMPORTANT
        req.end();
    });
    
    return promise.then((body) => {
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Hello World now', speechText)
                          .getResponse();
      }).catch(function (err) { console.log(err) } );
  
  }
}


const TasksIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'tasks';
  },
  handle(handlerInput) {
    var speechText = '';
    const http = require('https');
    var params = {
      host: '5h6o821oqg.execute-api.us-east-1.amazonaws.com',
      port: '443',
      path: '/Prod/next/OOO?sid='+handlerInput.requestEnvelope.session.sessionId 
    };
    //params.path=path+handlerInput.requestEnvelope.session.sessionId
    // this is a get, so there's no post data
    promise = new Promise((resolve, reject) => {
        var req = http.request(params, function(res) {
            // reject on bad status
            if (res.statusCode < 200 || res.statusCode >= 300) {
                return reject(new Error('statusCode=' + res.statusCode));
            }
            // cumulate data
            var body = [];
            res.on('data', function(chunk) {
                body.push(chunk);
                recipe = JSON.parse(body)
                speechText=recipe.task[0]
            });
            // resolve on end
            res.on('end', function() {
                try {
                    body = body;
                } catch(e) {
                    reject(e);
                }
                resolve(body);
            });
        });
        // reject on request error
        req.on('error', function(err) {
            // This is not a "Second reject", just a different sort of failure
            reject(err);
        });
        // IMPORTANT
        req.end();
    });
    
    return promise.then((body) => {
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .withSimpleCard('Recipe..', speechText)
                          .getResponse();
      }).catch(function (err) { console.log(err) } );
  
  },
};


const NextIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'next';
  },
  handle(handlerInput) {
    var speechText = '';
    const http = require('https');
    var params = {
      host: '5h6o821oqg.execute-api.us-east-1.amazonaws.com',
      port: '443',
      path: '/Prod/next/OOO?sid=asdf-asdf-asdf-asdf-asdf-12345' 
    };
    // this is a get, so there's no post data
    promise = new Promise((resolve, reject) => {
        var req = http.request(params, function(res) {
            // reject on bad status
            if (res.statusCode < 200 || res.statusCode >= 300) {
                return reject(new Error('statusCode=' + res.statusCode));
            }
            // cumulate data
            var body = [];
            res.on('data', function(chunk) {
                body.push(chunk);
                recipe = JSON.parse(body)
                console.log(recipe)
                speechText=recipe.task[0]
            });
            // resolve on end
            res.on('end', function() {
                try {
                    body = body;
                } catch(e) {
                    reject(e);
                }
                resolve(body);
            });
        });
        // reject on request error
        req.on('error', function(err) {
            // This is not a "Second reject", just a different sort of failure
            reject(err);
        });
        // IMPORTANT
        req.end();
    });
    
    return promise.then((body) => {
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .withSimpleCard('Recipe..', speechText)
                          .getResponse();
      }).catch(function (err) { console.log(err) } );
  
  },
};


const HelpIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'AMAZON.HelpIntent';
  },
  handle(handlerInput) {
    taskNum=taskNum+1
    const speechText = recipe.task[taskNum];

    return handlerInput.responseBuilder
      .speak(speechText)
      .reprompt(speechText)
      .withSimpleCard('Hello World', speechText)
      .getResponse();
  },
};

const CancelAndStopIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && (handlerInput.requestEnvelope.request.intent.name === 'AMAZON.CancelIntent'
        || handlerInput.requestEnvelope.request.intent.name === 'AMAZON.StopIntent');
  },
  handle(handlerInput) {
    const speechText = 'Goodbye!';

    return handlerInput.responseBuilder
      .speak(speechText)
      .withSimpleCard('Hello World', speechText)
      .getResponse();
  },
};

const SessionEndedRequestHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'SessionEndedRequest';
  },
  handle(handlerInput) {
    console.log(`Session ended with reason: ${handlerInput.requestEnvelope.request.reason}`);

    return handlerInput.responseBuilder.getResponse();
  },
};

const ErrorHandler = {
  canHandle() {
    return true;
  },
  handle(handlerInput, error) {
    console.log(`Error handled: ${error.message}`);

    return handlerInput.responseBuilder
      .speak('Sorry, I can\'t understand the command. Please say again.')
      .reprompt('Sorry, I can\'t understand the command. Please say again.')
      .getResponse();
  },
};

const skillBuilder = Alexa.SkillBuilders.custom();

exports.handler = skillBuilder
  .addRequestHandlers(
    LaunchRequestHandler,
    TasksIntentHandler,
    NextIntentHandler ,
    HelpIntentHandler,
    CancelAndStopIntentHandler,
    SessionEndedRequestHandler
  )
  .addErrorHandlers(ErrorHandler)
  .lambda();