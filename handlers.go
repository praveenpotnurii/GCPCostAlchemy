package main

import (
    "html/template"
    "net/http"
)

func loginHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method == http.MethodPost {
        username := r.FormValue("username")
        password := r.FormValue("password")
        // Here you would add authentication logic (e.g., check credentials)

        // For simplicity, let's just check if username and password are not empty
        if username != "" && password != "" {
            http.Redirect(w, r, "/welcome?username="+username, http.StatusSeeOther)
            return
        } else {
            http.Error(w, "Please enter both username and password.", http.StatusBadRequest)
            return
        }
    }

    // Display the login form
    tmpl, err := template.ParseFiles("templates/login.html")
    if err != nil {
        http.Error(w, "Error loading template", http.StatusInternalServerError)
        return
    }

    err = tmpl.Execute(w, nil)
    if err != nil {
        http.Error(w, "Error rendering template", http.StatusInternalServerError)
    }
}

