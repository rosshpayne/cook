# A Really Useful Cooking Skill

This Skill provides a virtual cooking assistant for cook book publishers, using a combination of Amazon Alexa voice assistant and intelligent display, with emphasis on the personalisation of recipe content.
The App facilitates searching for recipes across a users library of cook books, by recipe name, ingredient or keyword. Ingredient quantities can be scaled on-the-fly to suit a cook’s requirement and the ingredient listing displayed in the familiar format of the respective cook book. Clear and simple cooking instructions can be recited to the cook at their prompting. 


# So why another cooking skill?

There is no doubt about the enduringly popularity of cook books, as evident in their consistently stratospheric sales. Anyone with a modicum of interest in food loves to slowly browse through a good cook book given a moment, enticed by the photography, inspired by the recipe names and transported by the passion in their hands. The satisfying experience continues as you delve into a recipe. You study the ingredients and read the prose with a growing intent.
The difficulties starts, I find, when you move from the comfort of the reading space (both metaphorical and physical) into the kitchen. In brief, cooking while mentally tracking a list of ingredients, quantities and units and constantly reviewing a recipe text for instructions, with hands covered in flour and butter, is stressful and error prone, or at least for some of us, maybe even the majority, given the mountainous volume of cook books sold and the relative paucity of self- declared competent cooks.
What we need is a better way to deliver recipe content while in the kitchen. A hands free, voice activated virtual cook assistant, delivered by Alexa through my App, represents the best way to communicate a recipe to a cook. Succinct voice commands, when properly designed and delivered, will reduce the errors, stress and preparation time to as close to zero as possible.
The result is an App design that has deconstructed a recipe into some fifty unique data points, some sourced directly from the original recipe, others inferred and a good portion that are entirely new. When fully populated a recipe may contain between 250 to 650 data points depending on its complexity. The benefit of all this data are recipe formats that can be reimagined and presented to the cook in new and exciting ways starting with today’s voice assistants through to interactive intelligent displays of the future.
 
Now, instead of dashing between mixer and book with annoying regularity, the cook can request Alexa to recite a comprehensive set of cooking instructions. Each instruction designed to do one simple task, typically with one ingredient e.g. “measure 30 grams of caster sugar into a small bowl”. The fundamental benefit of a verbal assistant, when properly implemented, increases the confidence of an unskilled or average cook to tackle any recipe, no matter how complex or unfamiliar, with vastly reduced apprehension and preparation. The App however, can do much more than just recite a series of instructions.


# Features of the skill?
 
# Search for Recipes
The Alexa App has the facility to search for recipes based on keyword(s)3 (e.g. pasta, fish, flourless cake, vegetarian, desert etc), ingredient(s)4 or the full recipe name or part of the recipe name. For example, “search ricotta pudding” will return all recipes that have ricotta and pudding in the recipe name, “search tarragon ravioli” will display all recipes that have tarragon and ravioli in the name (in any order) and “search pasta” will list all recipes that have the keyword pasta associated with the recipe or pasta in the recipe name. To search for all chocolate cake recipes requires the cook to say “search chocolate cake” or “search chocolate cakes”.
# List Ingredients
Once a recipe has been selected, the user will be given the option to list the ingredient quantities. The format of the listing follows exactly the format adopted in the associated cook book.
If a recipe is divided into parts (e.g. filling, topping, garnish) then the listing will similarly be broken into parts.
Currently the ingredient listing is only displayed to the user, not recited, simply because I don’t see any value in this type of interaction, however this can be incorporated very easily.
I am currently investigating ways to send the ingredients listing to either the user’s printer or to the accompanying Alexa app6 on the users mobile, so they can consult the ingredient list when shopping.
# Scale a Recipe
This maybe a somewhat controversial feature but it comes from a personal requirement to reduce the ingredient quantities for some recipes. Consequently the App enables the user to verbally adjust all quantities to some fraction of the original using a single command, “scale <some percentage>”. This feature will only allow the user to scale down i.e. reduce the quantities. The list ingredients command will display all the ingredient quantities adjusted to their new values. Similarly, Alex will take into account the adjusted quantities when reciting the recipe instructions.
A scaling factor can be applied any number of times, including scaling back to the original quantities by saying “scale reset” or “scale one hundred percent”.
Importantly the App keeps the ingredient format consistent between adjustments. For example, if the current quantity is shown as a fraction, e.g. “1/2 cup”, and the scaling factor is 0.5 then the value displayed will be “1/4 cup” not “0.25 cup”. One litre, displayed as “1l”, under a 0.33 scaling will be displayed as “330ml”, rather than 0.33l, where rounding has adjusted it to the nearest 5ml7. However, for values less than “1/4 tsp” the App will replace the quantity with “a pinch”, which is not ideal but what can you do. Scaling a recipe will never be as accurate as the original, but in the vast majority of cases it proves more than satisfactory.
To give a visual context to the user, the header portion of the display shows the current scaling factor at all times.
The data model permits this feature to be disabled for an individual recipe if desired.
An alternative to directly scaling the ingredients is to adjust quantities based on a container size, which will be discussed next.
5 from a cost and performance perspective there is a delicate balance between storage size of the index and CPU and IO of the query logic. The preference has been to increase the index size so the CPU and IO can be reduced.
6 not to be confused with this App that runs solely on the Echo Show. The accompanying Alexa mobile app is an Amazon product that provides administrative functions to Alexa intelligent devices.
 
# Scale by Container Size
Some recipes, typically cakes, size the ingredient quantities to suite a particular container dimension. The “size” verbal command, enables a user to adjust all the ingredient quantities to suit the size of their container, provided the dimension is less than the original recipe container.
For example, a cake recipe may specify a 30 cm round cake tin, however the cook may only have a 26 cm round cake tin available, in which case the cook can say “size twenty six” and the App will multiple all quantities by 0.7511 (26**2/30**2). As with the scale command, the ingredient listing and recipe instructions will make use of the adjusted values.
This option can be disabled for an individual recipe if desired.

# List Containers and Utensils.
Cooking containers (bowls, cake tin, trays, etc) their size (small, medium, large) as well as cooking utensils (whisk, oven, electric mixer, rolling pin, grater etc) form a part of the recipe data, particularly relevant in the graph version of a recipe (more about that later).
The verbal command, “list containers” displays the number of containers8 and their size that a recipe requires. It also list the utensils used by the recipe.

# Recite Recipe Instructions
The principal use case of the App is to recite recipe instructions. It should be understood that instructions in this context, are not simply sentences recited from the text in a book, as this content does not lend itself to fail safe execution of a recipe, in my experience.
A single sentence from a book recipe may refer to many ingredients, often simply comma separated (up to five I have seen), requiring many separate activities across multiple containers to completely fulfil the intent of the sentence. A verbal instruction on the other hand, should represent a single non-divisible activity (within reason) applying to a single ingredient or mixture. Keeping an instruction as simple as possible reduces the chance of the cook making an error. Consequently one sentence in the original recipe text can easily expand to six or more verbal instructions.
Each verbal instruction should contain all the information required to complete it, such as the ingredient quantity, size, container dimensions or oven temperature when relevant. The user should not need to consult any other data source to fulfil a verbal instruction.
Alexa will start reciting an instruction with the command “list instructions”. Alexa will wait for up to twenty minutes for the user to respond with either “next”, “say again” or “previous”9. Previous will get Alexa to recite the previous instruction. The user can also respond with “list containers” or “list ingredients” should they need to review the containers or ingredients for any reason.

# Recipe Parts
Some recipes fall naturally into separate components which in turn are represented by their own recipe. It follows that “parts” have their own ingredient listing and set of cooking instructions. 

# Two Interfaces
The App supports both a graphic user interface (GUI) via the touch screen of the Echo Show, and voice commands via Alexa, to navigate through the various features of the app. The App is intended however, to be used via voice commands most if not all of the time, as this permits, not only hands free access to each feature, but also direct access to said feature. The GUI on the other hand, must navigate a hierarchy of screens to gain access to a feature and is therefore a lot slower.

# Session State
The App maintains a cooks session state for up to three days. This enables the cook to relaunch the App within this period and resume from exactly where they left off previously.
