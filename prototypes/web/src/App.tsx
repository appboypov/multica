import { Theme } from './settings/types';
import { MulticaSteerAllWorkPrototype } from './components/generated/MulticaSteerAllWorkPrototype';

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
      <MulticaSteerAllWorkPrototype />
    </>
  ); // %EXPORT_STATEMENT%
}

export default App;
