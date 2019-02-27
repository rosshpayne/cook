

module.exports = (header, subhdr, dataA, dataB, dataC) => { return {
    type: 'Alexa.Presentation.APL.RenderDocument',
    token: 'splash-screen',
    document: {
      type: 'APL',
      version: '1.0',
      import: [
        {
          name: 'alexa-styles',
          version: '1.0.0'
        },
        {
          name: 'alexa-layouts',
          version: '1.0.0'
        }
      ],
      resources:  [],
      styles: {
        textStylePressable: {
          values: [
            { backgroundColor: "blue",
              borderColor: "yellow",
              color: "yellow"
            }
        ]
      }
      },
      mainTemplate: {
        parameters: ['payload'],
        items: {
          when: "${@viewportProfile != @hubRoundSmall}",
          type: "Container",
          height: "100vh",
          width: "100vw",
          direction: "column",
          items: [
              {
              type: "AlexaHeader",
              headerTitle: header,
              headerSubtitle: subhdr,
              headerBackgroundColor: "red",
              headerBackButton: true,
              headerNavigationAction: "backButton"
              },
              {
              type: "Sequence",
              scrollDirection: "vertical",
              numbered: true,
              grow: 1,
              shrink: 1,
              width: "100vw",
              height: "100vh",
              items: [
                        {
                        type: "Container",
                        direction: "column",
                        data: "${payload.listdata.properties.dataA}",
                        spacing: 4,
                        height: "7vh",
                        alignItems: "left",
                        justifyContent: "end",
                        items: [
                                          {
                                          type: "Text",
                                          text: "${data.Title}",
                                          grow: 0,
                                          shrink: 0,
                                          fontSize: "15dp"
                                          }
                                          ] 
                        },
                        {
                        type: "Frame",
                        borderColor: "red",
                        borderWidth: 2,
                        height: "11vh",  
                        item: {
                              type: "Container",
                              direction: "column",
                              data: "${payload.listdata.properties.dataB}",
                              spacing: 4,
                              alignItems: "left",
                              height: "10vh",
                              justifyContent: "center",
                              items: [
                                      {
                                      type: "Text",
                                      text: "  ${data.Title}",
                                      grow: 0,
                                      shrink: 0,
                                      fontSize: "17dp",
                                      style: "textStylePrimary2"
                                      }
                                     ] 
                        }
                        },
                        {
                        type: "Container",
                        direction: "column",
                        data: "${payload.listdata.properties.dataC}",
                        spacing: 4,
                        height: "79vh",
                        alignItems: "left",
                        items: [
                                          {
                                          type: "Text",
                                          text: "${data.Title}",
                                          grow: 0,
                                          shrink: 0,
                                          fontSize: "15dp"
                                          }
                                          ] 
                        }
                        ]
              },
              {
                type: "AlexaFooter",
                hintText: "hint text goes here.."
              }
          ]
      }
      }
    },
    datasources: {
      listdata :    {        
            type: "object",
            properties: { 
              dataA,
              dataB,
              dataC
            }
      }
    }
  };
};