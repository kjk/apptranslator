// extract formatting string modifiers (%d, %s etc.) from both s1
// and s2 and return true if they are in the same order
function stringFormattingModifiersAreSame(s1, s2) {
	var a1 = extractFormattingModifiers(s1);
	var a2 = extractFormattingModifiers(s2);
	if (a1.length != a2.length) { return false; }
	var l = a1.length;
	for (var i=0; i<a1.length; i++) {
		if (a1[i] != a2[i]) { return false; }
	}
	return true;
}

// extract string formatting instructions (%s, %d etc.) from s 
// and return as an array
function extractFormattingModifiers(s) {
	var res = new Array;
	for (var i=0; i<s.length; i++) {
		var c = s[i];
		if (c == '%') {
			res.push(s[++i]);
		}
	}
	return res;
}

// $translation of a given $text can be submitted if it's not an empty
// string and if their string formatting instructions (%d, %s etc.) match
function canSubmitTranslation(text, translation) {
	var t = $.trim(translation);
	if (t.length == 0) { return false; }
	return stringFormattingModifiersAreSame(text, translation);
}