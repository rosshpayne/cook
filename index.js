const Alexa = require('ask-sdk-core');
var AWS = require('aws-sdk');


//var   recipe = { Response :[] };
var   invokeParams = {
        FunctionName: 'apigw-lambda-stack-3-TestFunction-1W27R33Q8ONM2',
        Qualifier: 'dev'
};

var lambda = new AWS.Lambda();
AWS.config.region = 'us-east-1';


const LaunchRequestHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'LaunchRequest';
  },
  
  handle(handlerInput) {
    var speechText = "Hi. To find a recipe, simply search for a keyword by saying 'search keyword or recipe name' where keyword is any ingredient or ingredients related to the recipe ";
    var displayText =  "Hi. To find a recipe, simply search for a keyword by saying 'search keyword or recipe name' where keyword is any ingredient or ingredients related to the recipe ";

        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Recipe World', displayText)
                          .getResponse();
  },
};

const EventHandler = {
  canHandle: handlerInput =>
    handlerInput.requestEnvelope.request.type === 'Alexa.Presentation.APL.UserEvent',
  
  handle: handlerInput => {
    const sid='sid='+handlerInput.requestEnvelope.session.sessionId ;

    const args = handlerInput.requestEnvelope.request.arguments;
    const event = args[0];
    //const ordinal = args[1];
    //const data = args[2];
    const selid='&sId='+args[1];

    switch (event) {
    case 'select':
      invokeParams.Payload = '{ "Path" : "select" ,"Param" : "'+sid+selid+'" }';

      promise = new Promise((resolve, reject) => {
          lambda.invoke(invokeParams, function(err, data) {
          if (err) {
            reject(err);
          } else {
            resolve(data.Payload);  }
          });
        });
    

    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
        
    case 'backButton':
       invokeParams.Payload = '{ "Path" : "back" ,"Param" : "'+sid+'" }';
        
       promise = new Promise((resolve, reject) => {
          lambda.invoke(invokeParams, function(err, data) {
          if (err) {
            reject(err);
          } else {
            resolve(data.Payload);  }
          });
        });
    
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
    } 
  },
};

const ScaleIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'ScaleIntent';
  },
  handle(handlerInput) {
    const querystring = require('querystring');
    //TODO is querystring necessary here as I believe AWS may escape it.
   // const srch='&srch='+querystring.escape(handlerInput.requestEnvelope.request.intent.slots.ingrdcat.resolutions.resolutionsPerAuthority[0].values[0].value.name);
   
    const sid="sid="+handlerInput.requestEnvelope.session.sessionId;
    const frac='&frac='+querystring.escape(handlerInput.requestEnvelope.request.intent.slots.fraction.resolutions.resolutionsPerAuthority[0].values[0].value.id);
    //                  handlerInput.requestEnvelope.request.intent.slots.YesNo.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    invokeParams.Payload = '{ "Path" : "scale" ,"Param" : "'+sid+frac+'" }';
    
    promise = new Promise((resolve, reject) => {
          lambda.invoke(invokeParams, function(err, data) {
          if (err) {
            reject(err);
          } else {
            resolve(data.Payload);  }
          });
        });    
        
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
    
  },
};

const DimensionIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'DimensionIntent';
  },
  handle(handlerInput) {
    const sid="sid="+handlerInput.requestEnvelope.session.sessionId;
    const dim='&dim='+handlerInput.requestEnvelope.request.intent.slots.size.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    //                  handlerInput.requestEnvelope.request.intent.slots.YesNo.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    invokeParams.Payload = '{ "Path" : "dimension" ,"Param" : "'+sid+dim+'" }';
    
    promise = new Promise((resolve, reject) => {
          lambda.invoke(invokeParams, function(err, data) {
          if (err) {
            reject(err);
          } else {
            resolve(data.Payload);  }
          });
        });    
        
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
    
  },
};

const BookIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'BookIntent';
  },
  handle(handlerInput) {
    const sid="sid="+handlerInput.requestEnvelope.session.sessionId;
    const bkid='&bkid='+handlerInput.requestEnvelope.request.intent.slots.BookName.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    invokeParams.Payload = '{ "Path" : "book" ,"Param" : "'+sid+bkid+'" }';
    
    promise = new Promise((resolve, reject) => {
          lambda.invoke(invokeParams, function(err, data) {
          if (err) {
            reject(err);
          } else {
            resolve(data.Payload);  }
          });
        });
    
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        if (resp.Type === "header") {
          const display = require('APL/header.js');
          return  handlerInput.responseBuilder
                            .speak(resp.Verbal)
                            .reprompt(resp.Verbal)
                            .addDirective(display(resp.BackBtn, resp.Header, resp.SubHdr, resp.List))
                            .getResponse();
        } else if (resp.Type === "Select") {
          const select = require('APL/select.js');
          return  handlerInput.responseBuilder
                            .speak(resp.Verbal)
                            .reprompt(resp.Verbal)
                            .addDirective(select(resp.BackBtn, resp.Header,resp.SubHdr, resp.List))
                            .getResponse(); 
        }
        }).catch(function (err) { console.log(err, err.stack);  } );
  },
};


const CloseBookIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'CloseBookIntent';
  },
  handle(handlerInput) {
    var sid="sid="+handlerInput.requestEnvelope.session.sessionId;
    invokeParams.Payload = '{ "Path" : "book/close" ,"Param" : "'+sid+'" }';

    promise = new Promise((resolve, reject) => {
          lambda.invoke(invokeParams, function(err, data) {
          if (err) {
            reject(err);
          } else {
            resolve(data.Payload);  }
          });
        });
    
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        if (resp.Type === "header") {
          const display = require('APL/header.js');
          return  handlerInput.responseBuilder
                            .speak(resp.Verbal)
                            .reprompt(resp.Verbal)
                            .addDirective(display(resp.Header,resp.SubHdr,  resp.List) )
                            .getResponse(); 
        } else if (resp.Type === "Select") {
          const select = require('APL/select.js');
          return  handlerInput.responseBuilder
                            .speak(resp.Verbal)
                            .reprompt(resp.Verbal)
                            .addDirective(select(resp.BackBtn, resp.Header,resp.SubHdr, resp.List))
                            .getResponse(); 
        }
        }).catch(function (err) { console.log(err, err.stack);  } );
  },
};


const SearchIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'SearchIntent';
  },
  handle(handlerInput) {
    const querystring = require('querystring');
    //TODO is querystring necessary here as I believe AWS may escape it.
   // const srch='&srch='+querystring.escape(handlerInput.requestEnvelope.request.intent.slots.ingrdcat.resolutions.resolutionsPerAuthority[0].values[0].value.name);
    const srch='&srch='+querystring.escape(handlerInput.requestEnvelope.request.intent.slots.ingrdcat.value);
    const sid="sid="+handlerInput.requestEnvelope.session.sessionId;
    invokeParams.Payload = '{ "Path" : "search" ,"Param" : "'+sid+srch+'" }';

    promise = new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
    
  },
};

const ResumeIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'ResumeIntent';
  },
  handle(handlerInput) {
    const sid="sid="+handlerInput.requestEnvelope.session.sessionId;
    invokeParams.Payload = '{ "Path" : "resume" ,"Param" : "'+sid+'" }';

    promise = new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
    
  },
};
const BackIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'BackIntent';
  },
  handle(handlerInput) {
    const select = require('APL/select.js');
    const ingredient = require('APL/ingredients.js');
    const sid='sid='+handlerInput.requestEnvelope.session.sessionId   ; 
    invokeParams.Payload = '{ "Path" : "back" ,"Param" : "'+sid+'" }';

    promise = new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        if (resp.Type === "Ingredient") {
          return  handlerInput.responseBuilder
                            .speak(resp.Verbal)
                            .reprompt(resp.Verbal)
                            .addDirective(ingredient(resp.Header,resp.SubHdr, resp.List))
                            .getResponse();
        } else {
          return  handlerInput.responseBuilder
                            .speak(resp.Verbal)
                            .reprompt(resp.Verbal)
                            .addDirective(select(resp.Header,resp.SubHdr, resp.List))
                            .getResponse();     
        }
      }).catch(function (err) { console.log(err, err.stack);  } );
  
  },
};


const SelectIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'SelectIntent';
  },
  handle(handlerInput) {
    const sid='sid='+handlerInput.requestEnvelope.session.sessionId   ; 
    const selid='&sId='+handlerInput.requestEnvelope.request.intent.slots.integerValue.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    invokeParams.Payload = '{ "Path" : "select" ,"Param" : "'+sid+selid+'" }';

    promise = new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
  
  },
};

const GotoIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'GotoIntent';
  },

  handle(handlerInput) {
    const sid='sid='+handlerInput.requestEnvelope.session.sessionId;
    const goId='&goId='+handlerInput.requestEnvelope.request.intent.slots.gotoId.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    invokeParams.Payload = '{ "Path" : "goto" ,"Param" : "'+sid+goId+'" }';

    promise = new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
  
  },
};

const TestIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'TestIntent';
  },
  handle(handlerInput) {
    const test = require('APL/test.js');
    const speakcmd = require('APL/testspeakcmd.js');

    return  handlerInput.responseBuilder
                            .reprompt("testing")
                            .addDirective(test("Header","SubHdr", resp.output))
                            .addDirective(speakcmd())
                            .getResponse(); 
  },
};


const NextIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'NextIntent';
  },
  handle(handlerInput) {
    const sid='sid='+handlerInput.requestEnvelope.session.sessionId;
    invokeParams.Payload = '{ "Path" : "next" ,"Param" : "'+sid+'" }';

    promise = new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
  },
};


const RepeatIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'RepeatIntent';
  },
  handle(handlerInput) {
    const sid='sid='+handlerInput.requestEnvelope.session.sessionId;
    invokeParams.Payload = '{ "Path" : "repeat" ,"Param" : "'+sid+'" }';

    promise = new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
  
  },
};

const PrevIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'PrevIntent';
  },
  handle(handlerInput) {
    const sid='sid='+handlerInput.requestEnvelope.session.sessionId;
    invokeParams.Payload = '{ "Path" : "prev" ,"Param" : "'+sid+'" }';

    promise = new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
        var  resp = JSON.parse(body);
        console.log(resp);
        return handleResponse(handlerInput, resp);
      }).catch(function (err) { console.log(err, err.stack);  } );
  },
};

const RecipeIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'RecipeIntent';
  },
  handle(handlerInput) {
    var speechText;
    var displayText;
    var recipe ;
    const querystring = require("querystring");
    //const bkIdRId = handlerInput.requestEnvelope.request.intent.slots.recipe.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    const rname = querystring.escape(handlerInput.requestEnvelope.request.intent.slots.recipe.resolutions.resolutionsPerAuthority[0].values[0].value.name);
    const sid='sid='+handlerInput.requestEnvelope.session.sessionId;
    //var path_='&rcp='+bkIdRId
    //if ( bkIdRId.length == 0 ) {
      path_='&rcp='+rname;
    //}
    invokeParams.Payload = '{ "Path" : "recipe" ,"Param" : "'+sid+path_+'" }';

    promise = new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
          recipe = JSON.parse(body);
          speechText=recipe.Text ;
          displayText = recipe.Verbal;
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', displayText)
                          .getResponse();
      }).catch(function (err) { console.log(err, err.stack);  } );
  },
};

const VersionIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'VersionIntent';
  },
  handle(handlerInput) {
    var speechText;
    var displayText;
    var recipe ;
    const sid='sid='+handlerInput.requestEnvelope.session.sessionId;
    const ver='&ver='+handlerInput.requestEnvelope.request.intent.slots.version.resolutions.resolutionsPerAuthority[0].values[0].value.name;
    invokeParams.Payload = '{ "Path" : "version" ,"Param" : "'+sid+ver+'" }';

    promise = new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
          recipe = JSON.parse(body);
          speechText=recipe.Text ;
          displayText = recipe.Verbal;
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', displayText)
                          .getResponse();
      }).catch(function (err) { console.log(err, err.stack);  } );
  },
};




const YesNoIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'YesNoIntent';
  },
  handle(handlerInput) {
    var speechText;
    var displayText;
    var recipe ;
    var sid="sid="+handlerInput.requestEnvelope.session.sessionId;
    var yesno='&yn='+handlerInput.requestEnvelope.request.intent.slots.YesNo.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    invokeParams.Payload = '{ "Path" : "yesno" ,"Param" : "'+sid+yesno+'" }';
 
    promise = new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
          recipe = JSON.parse(body);
          speechText=recipe.Text ;
          displayText = recipe.Verbal;
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', displayText)
                          .getResponse();
      }).catch(function (err) { console.log(err, err.stack);  } );
  },
};


const TaskIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'TaskIntent';
  },
  handle(handlerInput) {
    var speechText;
    var displayText;
    var recipe ;
    var sid="sid="+handlerInput.requestEnvelope.session.sessionId;
    invokeParams.Payload = '{ "Path" : "task" ,"Param" : "'+sid+'" }';

    promise = new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
          recipe = JSON.parse(body);
          speechText=recipe.Text ;
          displayText = recipe.Verbal;
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', displayText)
                          .getResponse();
      }).catch(function (err) { console.log(err, err.stack);  } );
  },
};

const ContainerIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'ContainerIntent';
  },
  handle(handlerInput) {
        var speechText;
    var displayText;
    var recipe ;
    var sid="sid="+handlerInput.requestEnvelope.session.sessionId;
    invokeParams.Payload = '{ "Path" : "container" ,"Param" : "'+sid+'" }';

    promise = new Promise((resolve, reject) => {
      lambda.invoke(invokeParams, function(err, data) {
        if (err) {
          reject(err);
        } else {
          resolve(data.Payload);  }
        });
    });
    
    return promise.then((body) => {
          recipe = JSON.parse(body);
          speechText=recipe.Text ;
          displayText = recipe.Verbal;
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', displayText)
                          .getResponse();
      }).catch(function (err) { console.log(err, err.stack);  } );
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


function handleResponse (handlerInput , resp) {
        if (resp.Type === "Ingredient") {
          const ingredient = require('APL/ingredients.js');
          return  handlerInput.responseBuilder
                            .speak(resp.Verbal)
                            .reprompt(resp.Verbal)
                            .addDirective(ingredient(resp.Header,resp.SubHdr, resp.List))
                            .getResponse();
       } else if (resp.Type === "Tripple") { 
          const tripple = require('APL/tripple3.js');
          const speakcmd = require('APL/speakcmd.js');
          return  handlerInput.responseBuilder
                            .reprompt(resp.Verbal)
                            .addDirective(tripple(resp.Header,resp.SubHdr, resp.ListA, resp.ListB, resp.ListC, resp.Verbal, resp.Text))
                            .addDirective(speakcmd())
                            .getResponse();  
       } else if (resp.Type === "Tripple2") { 
          const tripple = require('APL/tripple2.js');
          const speakcmd = require('APL/speakcmd.js');
          return  handlerInput.responseBuilder
                            .speak("here in tripple")
                            .reprompt(resp.Verbal)
                            .addDirective(tripple(resp.Header,resp.SubHdr, resp.ListA, resp.ListB, resp.ListC, resp.Verbal, resp.Text))
                            .addDirective(speakcmd())
                            .getResponse(); 
       } else if (resp.Type.indexOf("threaded") === 0 ) { 
          const tripple = require('APL/'+resp.Type+'.js');
          const speakcmd = require('APL/speakcmd.js');
          return  handlerInput.responseBuilder
                            .reprompt(resp.Verbal)
                            .addDirective(tripple(resp.Header,resp.SubHdr, resp.ListA, resp.ListB, resp.ListC, resp.Verbal, resp.Text, resp.ListD, resp.ListE, resp.ListF, resp.Thread1, resp.Thread2, resp.Color1, resp.Color2))
                            .addDirective(speakcmd())
                            .getResponse(); 
        } else if (resp.Type === "Select"){
          const select = require('APL/select.js');
          return  handlerInput.responseBuilder
                            .speak(resp.Verbal)
                            .reprompt(resp.Verbal)
                            .addDirective(select(resp.BackBtn, resp.Header,resp.SubHdr, resp.List))
                            .getResponse();     
        } else if (resp.Type === "Search") {
          const search = require('APL/search.js');
          return  handlerInput.responseBuilder
                          .speak(resp.Verbal)
                          .reprompt(resp.Text)
                          .addDirective(search(resp.BackBtn, resp.Header, resp.SubHdr, resp.List))
                          .getResponse();
        } else {
           const search = require('APL/search.js');
           return  handlerInput.responseBuilder
                          .speak(resp.Verbal)
                          .reprompt(resp.Text)
                          .addDirective(search(resp.Header, resp.SubHdr, resp.List))
                          .getResponse(); 
        }
}


const skillBuilder = Alexa.SkillBuilders.custom();

exports.handler = skillBuilder
  .addRequestHandlers(
    LaunchRequestHandler,
    ResumeIntentHandler,
    BookIntentHandler,
    BackIntentHandler,
    TestIntentHandler,
    CloseBookIntentHandler,
    RecipeIntentHandler,
    VersionIntentHandler,
    TaskIntentHandler,
    NextIntentHandler,
    YesNoIntentHandler,
    GotoIntentHandler,
    SelectIntentHandler,
    DimensionIntentHandler,
    ScaleIntentHandler,
    SearchIntentHandler,
    PrevIntentHandler,
    RepeatIntentHandler,
    ContainerIntentHandler,
    HelpIntentHandler,
    CancelAndStopIntentHandler,
    SessionEndedRequestHandler,
    EventHandler
  )
  .addErrorHandlers(ErrorHandler)
  .lambda();