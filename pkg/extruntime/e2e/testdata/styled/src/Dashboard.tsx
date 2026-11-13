import { Button, Tabs, TabsContent, TabsList, TabsTrigger } from "@iota-uz/sdk/solid-ui"
import styles from "./Dashboard.module.css"

export default function Dashboard() {
  return (
    <main data-testid="styled-dashboard" class={styles.panel}>
      <div data-testid="utility-box" class="p-4 bg-primary-600">
        Utilities on the SDK tokens
      </div>
      <div data-testid="module-box" class={styles.accent}>
        A scoped CSS module
      </div>
      <Button data-testid="kit-button">Save</Button>
      <Tabs defaultValue="terms">
        <TabsList>
          <TabsTrigger value="terms" data-testid="tab-terms">
            Terms
          </TabsTrigger>
          <TabsTrigger value="claims" data-testid="tab-claims">
            Claims
          </TabsTrigger>
        </TabsList>
        <TabsContent value="terms">
          <p data-testid="tab-body-terms">Treaty terms</p>
        </TabsContent>
        <TabsContent value="claims">
          <p data-testid="tab-body-claims">Claims history</p>
        </TabsContent>
      </Tabs>
    </main>
  )
}
