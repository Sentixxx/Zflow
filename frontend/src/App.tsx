import { Route, Switch } from "wouter";
import { ReaderPage } from "@/pages";

export function App() {
  return (
    <Switch>
      <Route path="/">{() => <ReaderPage />}</Route>
      <Route path="/settings">{() => <ReaderPage initialSettingsOpen={true} />}</Route>
      <Route>{() => <ReaderPage />}</Route>
    </Switch>
  );
}
  
