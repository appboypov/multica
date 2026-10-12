import { Theme } from './settings/types';
import { QuestionsPrototype } from './components/generated/QuestionsPrototype';

let theme: Theme = 'light';

function App() {
  function setTheme(theme: Theme) {
    if (theme === 'dark') {
      document.documentElement.classList.add('dark');
    } else {
      document.documentElement.classList.remove('dark');
    }
  }

  setTheme(theme);

  return (
    <>
      <QuestionsPrototype />
    </>
  ); // %EXPORT_STATEMENT%
}

export default App;
