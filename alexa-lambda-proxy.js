/* eslint-disable  func-names */
/* eslint-disable  no-console */

const Alexa = require('ask-sdk-core');
var AWS = require('aws-sdk');

var   recipe = { Response :[] };
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

    lambda.invoke(invokeParams, function(err, data) {
      if (err) {
        console.log(err, err.stack); // an error occurred
      } else {
        console.log(data.Payload);  
        recipe = JSON.parse(data.Payload);
        speechText=recipe.Text ;
        displayText = recipe.Verbal;
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', displayText)
                          .getResponse();
      }
      });
  
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

    lambda.invoke(invokeParams, function(err, data) {
      if (err) {
        console.log(err, err.stack); // an error occurred
      } else {
        console.log(data.Payload);  
        recipe = JSON.parse(data.Payload);
        speechText=recipe.Text ;
        displayText = recipe.Verbal;
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', displayText)
                          .getResponse();
      }
      });
  
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

    lambda.invoke(invokeParams, function(err, data) {
      if (err) {
        console.log(err, err.stack); // an error occurred
      } else {
        console.log(data.Payload);  
        recipe = JSON.parse(data.Payload);
        speechText=recipe.Text ;
        displayText = recipe.Verbal;
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', displayText)
                          .getResponse();
      }
      });
  
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

    lambda.invoke(invokeParams, function(err, data) {
      if (err) {
        console.log(err, err.stack); // an error occurred
      } else {
        console.log(data.Payload);  
        recipe = JSON.parse(data.Payload);
        speechText=recipe.Text ;
        displayText = recipe.Verbal;
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', displayText)
                          .getResponse();
      }
      });
  
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

    lambda.invoke(invokeParams, function(err, data) {
      if (err) {
        console.log(err, err.stack); // an error occurred
      } else {
        console.log(data.Payload);  
        recipe = JSON.parse(data.Payload);
        speechText=recipe.Text ;
        displayText = recipe.Verbal;
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', displayText)
                          .getResponse();
      }
      });
  
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
    const bkIdRId = handlerInput.requestEnvelope.request.intent.slots.recipe.resolutions.resolutionsPerAuthority[0].values[0].value.id;
    const rname = querystring.escape(handlerInput.requestEnvelope.request.intent.slots.recipe.resolutions.resolutionsPerAuthority[0].values[0].value.name);
    const sid='sid='+handlerInput.requestEnvelope.session.sessionId;
    var path_='&rcp='+bkIdRId
    if ( bkIdRId.length == 0 ) {
      path_='&rcp='+querystring.escape(rname);
    }
    invokeParams.Payload = '{ "Path" : "recipe ,"Param" : "'+sid+path_+'" }';

    lambda.invoke(invokeParams, function(err, data) {
      if (err) {
        console.log(err, err.stack); // an error occurred
      } else {
        console.log(data.Payload);  
        recipe = JSON.parse(data.Payload);
        speechText=recipe.Text ;
        displayText = recipe.Verbal;
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', displayText)
                          .getResponse();
      }
      });
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
    var recipe ;
    //TODO is querystring necessary here as I believe AWS may escape it.
    const srch='&srch='+querystring.escape(handlerInput.requestEnvelope.request.intent.slots.ingrdcat.resolutions.resolutionsPerAuthority[0].values[0].value.name);
    const sid="sid="+handlerInput.requestEnvelope.session.sessionId;
    invokeParams.Payload = '{ "Path" : "search" ,"Param" : "'+sid+srch+'" }';

    lambda.invoke(invokeParams, function(err, data) {
      if (err) {
        console.log(err, err.stack); // an error occurred
      } else {
        console.log(data.Payload);  
        recipe = JSON.parse(data.Payload);
        speechText=recipe.Text ;
        displayText = recipe.Verbal;
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', displayText)
                          .getResponse();
      }
      });
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

    lambda.invoke(invokeParams, function(err, data) {
      if (err) {
        console.log(err, err.stack); // an error occurred
      } else {
        console.log(data.Payload);  
        recipe = JSON.parse(data.Payload);
        speechText=recipe.Text ;
        displayText = recipe.Verbal;
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', displayText)
                          .getResponse();
      }
      });
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

    lambda.invoke(invokeParams, function(err, data) {
      if (err) {
        console.log(err, err.stack); // an error occurred
      } else {
        console.log(data.Payload);  
        recipe = JSON.parse(data.Payload);
        speechText=recipe.Text ;
        displayText = recipe.Verbal;
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', displayText)
                          .getResponse();
      }
      });
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

    lambda.invoke(invokeParams, function(err, data) {
      if (err) {
        console.log(err, err.stack); // an error occurred
      } else {
        console.log(data.Payload);  
        recipe = JSON.parse(data.Payload);
        speechText=recipe.Text ;
        displayText = recipe.Verbal;
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', displayText)
                          .getResponse();
      }
      });
  
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

    lambda.invoke(invokeParams, function(err, data) {
      if (err) {
        console.log(err, err.stack); // an error occurred
      } else {
        //console.log(data.Payload);  
        recipe = JSON.parse(data.Payload);
        speechText=recipe.Text ;
        displayText = recipe.Verbal;
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', displayText)
                          .getResponse();
      }
      });
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

    lambda.invoke(invokeParams, function(err, data) {
      if (err) {
        console.log(err, err.stack); // an error occurred
      } else {
        //console.log(data.Payload);  
        recipe = JSON.parse(data.Payload);
        speechText=recipe.Text ;
        displayText = recipe.Verbal;
        return  handlerInput.responseBuilder
                          .speak(speechText)
                          .reprompt(speechText)
                          .withSimpleCard('Instructions', displayText)
                          .getResponse();
      }
      });
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
    CloseBookIntentHandler,
    RecipeIntentHandler,
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
    SessionEndedRequestHandler
  )
  .addErrorHandlers(ErrorHandler)
  .lambda();