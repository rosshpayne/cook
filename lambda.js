/* eslint-disable  func-names */
/* eslint-disable  no-console */

const Alexa = require('ask-sdk-core');
var   recipe = { response :[] }
// Options to be used by request 

const LaunchRequestHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'LaunchRequest';
  },
  
  handle(handlerInput) {
    var speechText = 'Hi this is Helen. What book or recipe do you want me to open for you? Say book followed by its name or recipe followed by its name ';
        speechText=speechText
        console.log("console log write from lambda " )
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Hello World now', speechText)
                          .getResponse();

  
  },
}

const GotoIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'GotoIntent';
  },

  handle(handlerInput) {
    var speechText = '';
    const http = require('https');
    const path_='&goId='+handlerInput.requestEnvelope.request.intent.slots.gotoId.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    console.log(path_)
    var params = {
      host: '5h6o821oqg.execute-api.us-east-1.amazonaws.com',
      port: '443',
      path: '/Prod/goto?sid='+handlerInput.requestEnvelope.session.sessionId + path_
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
                speechText=recipe.response[0] + ' ' + recipe.response[2]
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
    speechText="You will need the following containers. "+speechText
    return promise.then((body) => {
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', speechText)
                          .getResponse();
      }).catch(function (err) { console.log(err) } );
  
  },
};


const NextIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'NextIntent';
  },
  handle(handlerInput) {
    var speechText = '';
    const http = require('https');
    var params = {
      host: '5h6o821oqg.execute-api.us-east-1.amazonaws.com',
      port: '443',
      path: '/Prod/next?sid='+handlerInput.requestEnvelope.session.sessionId 
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
                speechText=recipe.response[0] + ' ' + recipe.response[2]
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
    speechText="You will need the following containers. "+speechText
    return promise.then((body) => {
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', speechText)
                          .getResponse();
      }).catch(function (err) { console.log(err) } );
  
  },
};

const RepeatIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'RepeatIntent';
  },
  handle(handlerInput) {
    var speechText = '';
    const http = require('https');
    var params = {
      host: '5h6o821oqg.execute-api.us-east-1.amazonaws.com',
      port: '443',
      path: '/Prod/repeat?sid='+handlerInput.requestEnvelope.session.sessionId 
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
                speechText=recipe.response[0] + ' ' + recipe.response[2]
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
                          .withSimpleCard('Instructions', speechText)
                          .getResponse();
      }).catch(function (err) { console.log(err) } );
  
  },
};

const PrevIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'PrevIntent';
  },
  handle(handlerInput) {
    var speechText = '';
    const http = require('https');
    var params = {
      host: '5h6o821oqg.execute-api.us-east-1.amazonaws.com',
      port: '443',
      path: '/Prod/prev?sid='+handlerInput.requestEnvelope.session.sessionId 
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
                speechText=recipe.response[0] + ' ' + recipe.response[2]
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
                          .withSimpleCard('Instructions', speechText)
                          .getResponse();
      }).catch(function (err) { console.log(err) } );
  
  },
};

const RecipeIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'RecipeIntent';
  },
  handle(handlerInput) {
    var speechText = '';
    const http = require('https');
    //var querystring = require("querystring");
    
    //var bookname_ = handlerInput.requestEnvelope.request.intent.slots.recipe.value;
    const bkIdRId =handlerInput.requestEnvelope.request.intent.slots.recipe.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    //var  recipeEncoded = querystring.stringify({query: recipeName});
    var path_='&bkrid='+bkIdRId
    var params = {
      host: '5h6o821oqg.execute-api.us-east-1.amazonaws.com',
      port: '443',
      path: '/Prod/recipe?sid='+handlerInput.requestEnvelope.session.sessionId+path_
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
                speechText=recipe.response[0]+' '+recipe.response[2]
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
    speechText="You will need the following containers. "+speechText
    return promise.then((body) => {
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', speechText)
                          .getResponse();
      }).catch(function (err) { console.log(err) } );
  
  },
};

const BookIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'BookIntent';
  },
  handle(handlerInput) {
    var speechText = '';
    const http = require('https');
    const path_='&bkname='+handlerInput.requestEnvelope.request.intent.slots.BookName.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    var params = {
      host: '5h6o821oqg.execute-api.us-east-1.amazonaws.com',
      port: '443',
      path: '/Prod/book?sid='+handlerInput.requestEnvelope.session.sessionId+path_
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
                speechText=recipe.response[0] + ' ' + recipe.response[2]
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
    speechText="You will need the following containers. "+speechText
    return promise.then((body) => {
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', speechText)
                          .getResponse();
      }).catch(function (err) { console.log(err) } );
  
  },
};

const TaskIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'TaskIntent';
  },
  handle(handlerInput) {
    var speechText = '';
    const http = require('https');
    var params = {
      host: '5h6o821oqg.execute-api.us-east-1.amazonaws.com',
      port: '443',
      path: '/Prod/task?sid='+handlerInput.requestEnvelope.session.sessionId 
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
                speechText=recipe.response[0]+ ' ' + 'in task intent'; //recipe.task[0]
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
    speechText="You will need the following containers. "+speechText
    return promise.then((body) => {
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', speechText)
                          .getResponse();
      }).catch(function (err) { console.log(err) } );
  
  },
};

const ContainerIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'ContainerIntent';
  },
  handle(handlerInput) {
    var speechText = '';
    const http = require('https');
    var params = {
      host: '5h6o821oqg.execute-api.us-east-1.amazonaws.com',
      port: '443',
      path: '/Prod/container?sid='+handlerInput.requestEnvelope.session.sessionId 
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
                //console.log("body:"+body.String)
                recipe = JSON.parse(body)
                //console.log("recipe:"+recipe.String)
                //recipe = 'XXYYZZ'
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
    speechText="You will need the following containers. "+speechText
    return promise.then((body) => {
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Containers', speechText)
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
    const speechText = 'You can say hello to me!';

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
    BookIntentHandler,
    RecipeIntentHandler,
    TaskIntentHandler,
    NextIntentHandler,
    GotoIntentHandler,
    PrevIntentHandler,
    RepeatIntentHandler,
    ContainerIntentHandler,
    HelpIntentHandler,
    CancelAndStopIntentHandler,
    SessionEndedRequestHandler
  )
  .addErrorHandlers(ErrorHandler)
  .lambda();