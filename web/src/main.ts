import './styles/app.css';
import { mount } from 'svelte';
import App from './App.svelte';
import './lib/prefs.svelte'; // applies the saved theme, text size and width before the first paint

mount(App, { target: document.getElementById('app')! });
