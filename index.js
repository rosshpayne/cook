
const Alexa = require('ask-sdk-core');
var AWS = require('aws-sdk');
var main = require('./main.js');


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
    var speechText = 'Hi. Do you want to search for a recipe based on ingredient or open a recipe? Say "open at" to select a recipe or search for recipes by saying search for chocolate cake recipes to display a list of recipes of chocolate cakes.';
    var displayText = 'Hi. Do you want to search for a recipe based on ingredient or open a recipe? Say "open at" to select a recipe or search for recipes by saying search for chocolate cake recipes to display a list of recipes of chocolate cakes.';

        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Recipe World', displayText)
                          .getResponse();
  },
}

const SelectIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'SelectIntent';
  },
  handle(handlerInput) {
    const querystring = require('querystring');
    var speechText;
    var displayText;
    var recipe ;
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

const GotoIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'GotoIntent';
  },

  handle(handlerInput) {
    const querystring = require('querystring');
    var speechText;
    var displayText;
    var recipe ;
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


const NextIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'NextIntent';
  },
  handle(handlerInput) {
    const querystring = require('querystring');
    var speechText;
    var displayText;
    var recipe ;
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

const RepeatIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'RepeatIntent';
  },
  handle(handlerInput) {
    var speechText;
    var displayText;
    var recipe ;
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

const PrevIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'PrevIntent';
  },
  handle(handlerInput) {
    var speechText;
    var displayText;
    var recipe ;
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

const SearchIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'SearchIntent';
  },
  handle(handlerInput) {
    const querystring = require('querystring');
    var speechText;
    var displayText;
    var resp ;
    //TODO is querystring necessary here as I believe AWS may escape it.
    const srch='&srch='+querystring.escape(handlerInput.requestEnvelope.request.intent.slots.ingrdcat.resolutions.resolutionsPerAuthority[0].values[0].value.name);
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
          resp = JSON.parse(body);
          const data = {
            text: resp.Text,
            verbal: resp.Verbal,
            mchoice: resp.List
          }
        return  handlerInput.responseBuilder
                          .speak(resp.Text)
                          .reprompt(resp.Text)
                          .addDirective(main(data))
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

const BookIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'BookIntent';
  },
  handle(handlerInput) {
    var speechText;
    var displayText;
    var recipe ;
    const sid="sid="+handlerInput.requestEnvelope.session.sessionId;
    const bkid='&bkid='+handlerInput.requestEnvelope.request.intent.slots.BookName.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    invokeParams.Payload = '{ "Path" : "book" ,"Param" : "'+sid+bkid+'" }';
    //const data = '["Ross","Payne","gHij","klm","nopq","rst","uvw","xyz"] '
    //const data = "["+'"Ross",'+'"Payne"'+"]"
    const L1 = {
      Title: "Title1",
      SubTitle1: "SubTitle1..",
      SubTitle2: "SubTitle2...",
      Text: "more text here .."
    }
    const L2 = {
      Title: "Title2",
      SubTitle1: "SubTitle12..",
      SubTitle2: "SubTitle22...",
      Text: "more text here 2  .."
    }
    const L3 = {
      Title: "Title3",
      SubTitle1: "SubTitle13..",
      SubTitle2: "SubTitle23...",
      Text: "more text here 3  .."
    }
    const L4 = {
      Title: "Title4",
      SubTitle1: "SubTitle14..",
      SubTitle2: "SubTitle24...",
      Text: "more text here 4  .."
    }
    const L5 = {
      Title: "Title5",
      SubTitle1: "SubTitle15..",
      SubTitle2: "SubTitle25...",
      Text: "more text here 5  .."
    }
    const data = {
      text: "abcd",
      verbal: "defsdj",
      list: [L1, L2, L3, L4, L5]
    }
    
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
                          .addDirective(main(data))
                          .getResponse();
      }).catch(function (err) { console.log(err, err.stack);  } );
  },
};

const CloseBookIntentHandler = {
  canHandle(handlerInput) {
    return handlerInput.requestEnvelope.request.type === 'IntentRequest'
      && handlerInput.requestEnvelope.request.intent.name === 'CloseBookIntent';
  },
  handle(handlerInput) {
    var speechText;
    var displayText;
    var recipe ;
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

const EventHandler = {
  canHandle: handlerInput =>
    handlerInput.requestEnvelope.request.type === 'Alexa.Presentation.APL.UserEvent',
  
  handle: handlerInput => {
    const args = handlerInput.requestEnvelope.request.arguments;
    const event = args[0];
    const ordinal = args[1];
    const data = args[2];

    switch (event) {
    case 'ItemSelected':
        console.log(data)
        return handlerInput.responseBuilder
      .speak('Where would you like to explore ' + ordinal + ' ' + data)
      .reprompt('Where would you like to explore?')
      .getResponse();
    } 
    },
};

const skillBuilder = Alexa.SkillBuilders.custom();

exports.handler = skillBuilder
  .addRequestHandlers(
    LaunchRequestHandler,
    BookIntentHandler,
    CloseBookIntentHandler,
    RecipeIntentHandler,
    VersionIntentHandler,
    TaskIntentHandler,
    NextIntentHandler,
    YesNoIntentHandler,
    GotoIntentHandler,
    SelectIntentHandler,
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