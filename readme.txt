AppTranslator is a web app written in Go for collecting crowd-sourced
translations for software.

You can see it running at http://www.apptranslator.org.

The software was developed for SumatraPDF (http://blog.kowalczyk.info/software/sumatrapdf/).

You could run it for your own software, but it's a server side software, so
you would need to figure out your deployment strategy, backup and write a script
that uploads your strings to the server and a script that downloads the
translations from the server and updates your code to use those translations.
In other words, it's complicated.

For more information, see docs/deploy_your_own.txt
